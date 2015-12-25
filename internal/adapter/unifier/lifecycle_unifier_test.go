package unifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func TestLifecycleUnifier_StartStop(t *testing.T) {
	config := DefaultConfig()
	config.CleanupInterval = 100 * time.Millisecond
	logger := createTestLogger()

	unifier := NewLifecycleUnifier(config, logger).(*LifecycleUnifier)

	ctx := context.Background()

	// Test starting
	err := unifier.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, unifier.isRunning.Load())

	// Test double start
	err = unifier.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test stopping
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err = unifier.Stop(stopCtx)
	assert.NoError(t, err)
	assert.False(t, unifier.isRunning.Load())

	// Test double stop
	err = unifier.Stop(stopCtx)
	assert.NoError(t, err) // Should be idempotent
}

func TestLifecycleUnifier_EndpointStateTracking(t *testing.T) {
	config := DefaultConfig()
	config.MaxConsecutiveFailures = 2
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

	// Test recording failures
	unifier.RecordEndpointFailure(endpoint.URLString, errors.New("connection failed"))
	state := unifier.GetEndpointState(endpoint.URLString)
	assert.NotNil(t, state)
	assert.Equal(t, domain.EndpointStateDegraded, state.State)
	assert.Equal(t, 1, state.ConsecutiveFailures)

	// Second failure should mark endpoint as offline
	unifier.RecordEndpointFailure(endpoint.URLString, errors.New("connection failed again"))
	state = unifier.GetEndpointState(endpoint.URLString)
	assert.Equal(t, domain.EndpointStateOffline, state.State)
	assert.Equal(t, 2, state.ConsecutiveFailures)

	// Success should restore online state through UnifyModels call
	models := []*domain.ModelInfo{{Name: "test"}}
	_, err = unifier.UnifyModels(ctx, models, endpoint)
	state = unifier.GetEndpointState(endpoint.URLString)
	assert.Equal(t, domain.EndpointStateOnline, state.State)
	assert.Equal(t, 0, state.ConsecutiveFailures)
}

func TestLifecycleUnifier_ModelTTLCleanup(t *testing.T) {
	config := DefaultConfig()
	config.ModelTTL = 100 * time.Millisecond
	config.CleanupInterval = 50 * time.Millisecond
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

	// Add a model
	models := []*domain.ModelInfo{
		{
			Name: "test-model",
			Size: 1000,
			Details: &domain.ModelDetails{
				Digest: strPtr("abc123"),
				State:  strPtr("loaded"),
			},
		},
	}

	unified, err := unifier.UnifyModels(ctx, models, endpoint)
	assert.NoError(t, err)
	assert.Len(t, unified, 1)

	// Verify model exists
	model, err := unifier.ResolveModel(ctx, "test-model")
	assert.NoError(t, err)
	assert.NotNil(t, model)

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)
