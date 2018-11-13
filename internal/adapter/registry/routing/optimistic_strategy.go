package routing

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// OptimisticStrategy falls back to healthy endpoints when model not found
type OptimisticStrategy struct {
	logger           logger.StyledLogger
	fallbackBehavior string
}

// NewOptimisticStrategy creates a new optimistic routing strategy
func NewOptimisticStrategy(fallbackBehavior string, logger logger.StyledLogger) *OptimisticStrategy {
	if fallbackBehavior == "" {
		fallbackBehavior = constants.FallbackBehaviorCompatibleOnly
	}
	return &OptimisticStrategy{
		fallbackBehavior: fallbackBehavior,
		logger:           logger,
	}
}

// Name returns the strategy name
func (s *OptimisticStrategy) Name() string {
	return StrategyOptimistic
}
