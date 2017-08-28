package registry

import (
	"context"

	"github.com/putrkobey/Olla/internal/adapter/registry/routing"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// RoutingRegistry wraps a base registry with routing strategy support
type RoutingRegistry struct {
	domain.ModelRegistry
	routingStrategy ports.ModelRoutingStrategy
	logger          logger.StyledLogger
}

// NewRoutingRegistry creates a registry wrapper with routing support
func NewRoutingRegistry(base domain.ModelRegistry, routingConfig *config.ModelRoutingStrategy,
	discovery DiscoveryService, logger logger.StyledLogger) *RoutingRegistry {

	// create routing strategy
	var routingStrategy ports.ModelRoutingStrategy
	if routingConfig != nil {
		factory := routing.NewFactory(logger)
		// adapt discovery interface if provided
		var discoveryAdapter ports.DiscoveryService
		if discovery != nil {
			discoveryAdapter = &discoveryServiceAdapter{discovery: discovery}
		}

