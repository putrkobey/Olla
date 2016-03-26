package routing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// DiscoveryStrategy refreshes model discovery before deciding
type DiscoveryStrategy struct {
	discovery      ports.DiscoveryService
	logger         logger.StyledLogger
	strictFallback *StrictStrategy // use strict strategy after discovery
	options        config.ModelRoutingStrategyOptions
}

// NewDiscoveryStrategy creates a new discovery routing strategy
func NewDiscoveryStrategy(discovery ports.DiscoveryService, options config.ModelRoutingStrategyOptions, logger logger.StyledLogger) *DiscoveryStrategy {
	return &DiscoveryStrategy{
		discovery:      discovery,
		options:        options,
		logger:         logger,
		strictFallback: NewStrictStrategy(logger),
	}
}

// Name returns the strategy name
func (s *DiscoveryStrategy) Name() string {
	return StrategyDiscovery
}

// GetRoutableEndpoints refreshes discovery then routes based on updated model information
func (s *DiscoveryStrategy) GetRoutableEndpoints(
	ctx context.Context,
	modelName string,
	healthyEndpoints []*domain.Endpoint,
	modelEndpoints []string,
) ([]*domain.Endpoint, *domain.ModelRoutingDecision, error) {
	// first check if we already have healthy endpoints with the model
	modelEndpointMap := make(map[string]bool)
	for _, url := range modelEndpoints {
		modelEndpointMap[url] = true
	}

	var currentlyRoutable []*domain.Endpoint
	for _, endpoint := range healthyEndpoints {
		if modelEndpointMap[endpoint.URLString] {
			currentlyRoutable = append(currentlyRoutable, endpoint)
		}
	}

	// if we already have routable endpoints, use them
	if len(currentlyRoutable) > 0 {
		s.logger.Debug("Discovery strategy found existing endpoints with model",
			"model", modelName,
			"routable", len(currentlyRoutable))

		return currentlyRoutable, ports.NewRoutingDecision(
			s.Name(),
			ports.RoutingActionRouted,
			constants.RoutingReasonModelFoundNoRefresh,
		), nil
	}

	// no healthy endpoints have the model - trigger discovery refresh if configured
	if !s.options.DiscoveryRefreshOnMiss {
		s.logger.Debug("Discovery refresh disabled, rejecting request",
			"model", modelName)

		return nil, ports.NewRoutingDecision(
				s.Name(),
				ports.RoutingActionRejected,
				constants.RoutingReasonModelUnavailableNoRefresh,
			), domain.NewModelRoutingError(
				modelName,
				s.Name(),
				"rejected",
				len(healthyEndpoints),
				modelEndpoints,
				fmt.Errorf("model %s not available and discovery refresh disabled", modelName),
			)
	}

	discoveryTimeout := s.options.DiscoveryTimeout
	// Guard against a zero timeout from user config overrides — context.WithTimeout(ctx, 0) expires immediately.
	if discoveryTimeout <= 0 {
		s.logger.Warn("DiscoveryTimeout is zero or negative, defaulting to 2 seconds", "configured", discoveryTimeout)
		discoveryTimeout = 2 * time.Second
	}

	s.logger.Info("Triggering discovery refresh for model",
		"model", modelName,
		"timeout", discoveryTimeout)

	// create timeout context for discovery
	discoveryCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	// trigger discovery refresh
	startTime := time.Now()
	if err := s.discovery.RefreshEndpoints(discoveryCtx); err != nil {
		s.logger.Warn("Discovery refresh failed",
			"model", modelName,
