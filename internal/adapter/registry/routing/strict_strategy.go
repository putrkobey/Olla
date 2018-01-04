package routing

import (
	"context"
	"fmt"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// StrictStrategy only routes to endpoints that have the model
type StrictStrategy struct {
	logger logger.StyledLogger
}

// NewStrictStrategy creates a new strict routing strategy
func NewStrictStrategy(logger logger.StyledLogger) *StrictStrategy {
	return &StrictStrategy{
		logger: logger,
	}
}

// Name returns the strategy name
func (s *StrictStrategy) Name() string {
	return StrategyStrict
}

// GetRoutableEndpoints returns only healthy endpoints that have the model
func (s *StrictStrategy) GetRoutableEndpoints(
	ctx context.Context,
	modelName string,
	healthyEndpoints []*domain.Endpoint,
	modelEndpoints []string,
) ([]*domain.Endpoint, *domain.ModelRoutingDecision, error) {
	// no endpoints have the model
	if len(modelEndpoints) == 0 {
		s.logger.Debug("Model not found in any endpoint",
			"model", modelName,
			"healthy_endpoints", len(healthyEndpoints))

		return nil, ports.NewRoutingDecision(
				s.Name(),
				ports.RoutingActionRejected,
				constants.RoutingReasonModelNotFound,
			), domain.NewModelRoutingError(
				modelName,
				s.Name(),
				"rejected",
				len(healthyEndpoints),
