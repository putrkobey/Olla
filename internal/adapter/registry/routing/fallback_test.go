package routing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func createTestLogger() logger.StyledLogger {
	logCfg := &logger.Config{Level: "error"}
	log, _, _ := logger.New(logCfg)
	return logger.NewPlainStyledLogger(log)
}

func TestOptimisticStrategy_FallbackBehavior(t *testing.T) {
	ctx := context.Background()
	testLogger := createTestLogger()

	healthyEndpoints := []*domain.Endpoint{
		{Name: "ep1", URLString: "http://ep1", Status: domain.StatusHealthy},
		{Name: "ep2", URLString: "http://ep2", Status: domain.StatusHealthy},
	}

	modelEndpoints := []string{"http://ep3"} // Model only on unhealthy endpoint

	t.Run("compatible_only rejects when model not on healthy endpoints", func(t *testing.T) {
		strategy := &OptimisticStrategy{
			fallbackBehavior: constants.FallbackBehaviorCompatibleOnly,
			logger:           testLogger,
		}

		result, decision, err := strategy.GetRoutableEndpoints(ctx, "test-model", healthyEndpoints, modelEndpoints)

		assert.NoError(t, err)
		assert.Empty(t, result)
		assert.Equal(t, "rejected", string(decision.Action))
		assert.Equal(t, constants.RoutingReasonModelUnavailableCompatibleOnly, decision.Reason)
	})

	t.Run("none rejects when model not on healthy endpoints", func(t *testing.T) {
		strategy := &OptimisticStrategy{
			fallbackBehavior: constants.FallbackBehaviorNone,
			logger:           testLogger,
		}

		result, decision, err := strategy.GetRoutableEndpoints(ctx, "test-model", healthyEndpoints, modelEndpoints)

		assert.NoError(t, err)
		assert.Empty(t, result)
		assert.Equal(t, "rejected", string(decision.Action))
		assert.Equal(t, constants.RoutingReasonModelUnavailableNoFallback, decision.Reason)
	})

	t.Run("all returns all healthy when model not on healthy endpoints", func(t *testing.T) {
		strategy := &OptimisticStrategy{
			fallbackBehavior: constants.FallbackBehaviorAll,
			logger:           testLogger,
		}

		result, decision, err := strategy.GetRoutableEndpoints(ctx, "test-model", healthyEndpoints, modelEndpoints)

		assert.NoError(t, err)
		assert.Equal(t, healthyEndpoints, result)
		assert.Equal(t, "fallback", string(decision.Action))
		assert.Equal(t, constants.RoutingReasonAllHealthyFallback, decision.Reason)
	})

	t.Run("returns model endpoints when available", func(t *testing.T) {
		strategy := &OptimisticStrategy{
			fallbackBehavior: constants.FallbackBehaviorCompatibleOnly,
			logger:           testLogger,
		}

		// Model on healthy endpoint
		modelEndpointsHealthy := []string{"http://ep1"}

		result, decision, err := strategy.GetRoutableEndpoints(ctx, "test-model", healthyEndpoints, modelEndpointsHealthy)

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "ep1", result[0].Name)
		assert.Equal(t, "routed", string(decision.Action))
		assert.Equal(t, constants.RoutingReasonModelFound, decision.Reason)
	})
}

func TestDiscoveryStrategy_FallbackBehavior(t *testing.T) {
	ctx := context.Background()
	testLogger := createTestLogger()

	healthyEndpoints := []*domain.Endpoint{
		{Name: "ep1", URLString: "http://ep1", Status: domain.StatusHealthy},
		{Name: "ep2", URLString: "http://ep2", Status: domain.StatusHealthy},
	}

	modelEndpoints := []string{"http://ep3"} // Model only on unhealthy endpoint

	t.Run("compatible_only rejects after discovery when model not found", func(t *testing.T) {
		mockDiscovery := &mockDiscoveryForTest{
			healthyEndpoints: healthyEndpoints,
			shouldFail:       false,
		}

		strategy := &DiscoveryStrategy{
			discovery: mockDiscovery,
			options: config.ModelRoutingStrategyOptions{
				FallbackBehavior:       constants.FallbackBehaviorCompatibleOnly,
				DiscoveryRefreshOnMiss: true,
			},
