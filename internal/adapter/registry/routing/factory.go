package routing

import (
	"sync"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	StrategyStrict     = "strict"
	StrategyOptimistic = "optimistic"
	StrategyDiscovery  = "discovery"
)

// Factory creates model routing strategy instances
type Factory struct {
	creators map[string]func(config.ModelRoutingStrategyOptions, ports.DiscoveryService, logger.StyledLogger) ports.ModelRoutingStrategy
	logger   logger.StyledLogger
	mu       sync.RWMutex
}

// NewFactory creates a new routing strategy factory with default registrations
func NewFactory(log logger.StyledLogger) *Factory {
	factory := &Factory{
		creators: make(map[string]func(config.ModelRoutingStrategyOptions, ports.DiscoveryService, logger.StyledLogger) ports.ModelRoutingStrategy),
		logger:   log,
	}

	// register default strategies
	factory.Register(StrategyStrict, func(opts config.ModelRoutingStrategyOptions, discovery ports.DiscoveryService, log logger.StyledLogger) ports.ModelRoutingStrategy {
		return NewStrictStrategy(log)
	})

	factory.Register(StrategyOptimistic, func(opts config.ModelRoutingStrategyOptions, discovery ports.DiscoveryService, log logger.StyledLogger) ports.ModelRoutingStrategy {
		return NewOptimisticStrategy(opts.FallbackBehavior, log)
	})

