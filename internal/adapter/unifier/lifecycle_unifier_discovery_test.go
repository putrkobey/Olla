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
