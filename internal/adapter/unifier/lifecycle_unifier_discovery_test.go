package unifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// mockDiscoveryClient implements DiscoveryClient for testing
type mockDiscoveryClient struct {
	discoverFunc func(ctx context.Context, endpoint *domain.Endpoint) ([]*domain.ModelInfo, error)
	callCount    int
}

func (m *mockDiscoveryClient) DiscoverModels(ctx context.Context, endpoint *domain.Endpoint) ([]*domain.ModelInfo, error) {
	m.callCount++
	if m.discoverFunc != nil {
		return m.discoverFunc(ctx, endpoint)
	}
	return []*domain.ModelInfo{
		{
			Name: "test-model",
			Size: 1000,
			Details: &domain.ModelDetails{
				State: strPtr("loaded"),
			},
		},
	}, nil
}

func TestLifecycleUnifier_ForceEndpointCheck(t *testing.T) {
	config := DefaultConfig()
	logger := createTestLogger()
	unifier := NewLifecycleUnifier(config, logger).(*LifecycleUnifier)

	ctx := context.Background()
	err := unifier.Start(ctx)
	require.NoError(t, err)
	defer unifier.Stop(ctx)

	// Test without discovery client
	err = unifier.ForceEndpointCheck(ctx, "http://test-endpoint")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discovery client not configured")

	// Set up discovery client
	mockClient := &mockDiscoveryClient{
		discoverFunc: func(ctx context.Context, endpoint *domain.Endpoint) ([]*domain.ModelInfo, error) {
			return []*domain.ModelInfo{
				{
					Name: "test-model",
					Size: 1000,
					Details: &domain.ModelDetails{
						State: strPtr("loaded"),
					},
				},
			}, nil
		},
	}
	unifier.SetDiscoveryClient(mockClient)

	// Add an endpoint first
	endpoint := &domain.Endpoint{
		URLString: "http://test-endpoint",
		Name:      "test-endpoint",
	}
	models := []*domain.ModelInfo{
		{
			Name: "test-model",
			Size: 1000,
		},
	}
	_, err = unifier.UnifyModels(ctx, models, endpoint)
	assert.NoError(t, err)

	// Force check should succeed
	err = unifier.ForceEndpointCheck(ctx, "http://test-endpoint")
	assert.NoError(t, err)
	assert.Equal(t, 1, mockClient.callCount)

	// Test with discovery failure
	mockClient.discoverFunc = func(ctx context.Context, endpoint *domain.Endpoint) ([]*domain.ModelInfo, error) {
		return nil, errors.New("discovery failed")
	}

	err = unifier.ForceEndpointCheck(ctx, "http://test-endpoint")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint check failed")

	// Verify failure was recorded
	state := unifier.GetEndpointState("http://test-endpoint")
	assert.NotNil(t, state)
	assert.Equal(t, 1, state.ConsecutiveFailures)

	// Test with non-existent endpoint
	err = unifier.ForceEndpointCheck(ctx, "http://non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint not found")
}

func TestLifecycleUnifier_GetCircuitBreakerStats(t *testing.T) {
	config := DefaultConfig()
	config.CircuitBreaker.Enabled = true
	config.CircuitBreaker.FailureThreshold = 2
	config.MaxConsecutiveFailures = 3

	logger := createTestLogger()
	unifier := NewLifecycleUnifier(config, logger).(*LifecycleUnifier)

	ctx := context.Background()
	err := unifier.Start(ctx)
	require.NoError(t, err)
	defer unifier.Stop(ctx)

	endpoint := &domain.Endpoint{
		URLString: "http://test-endpoint",
		Name:      "test-endpoint",
	}

	// Initially no circuit breakers
	stats := unifier.GetCircuitBreakerStats()
	assert.Empty(t, stats)

	// Create circuit breaker by recording failure
	unifier.RecordEndpointFailure(endpoint.URLString, errors.New("test error"))

	// Should have stats now
	stats = unifier.GetCircuitBreakerStats()
	assert.Len(t, stats, 1)
	assert.Contains(t, stats, endpoint.URLString)
	assert.Equal(t, "closed", stats[endpoint.URLString].State)
	assert.Equal(t, 1, stats[endpoint.URLString].Failures)

	// Trip the circuit breaker
	unifier.RecordEndpointFailure(endpoint.URLString, errors.New("test error 2"))

