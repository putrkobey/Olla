package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/discovery"
	"github.com/putrkobey/Olla/internal/adapter/factory"
	"github.com/putrkobey/Olla/internal/adapter/health"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// DiscoveryService orchestrates endpoint discovery, health monitoring, and model
// cataloguing. It runs initial health checks during startup to ensure endpoints
// are validated before the proxy accepts traffic, preventing "no healthy endpoints"
// errors during the critical startup phase.
type DiscoveryService struct {
	config         *config.DiscoveryConfig
	registryConfig *config.ModelRegistryConfig
	healthChecker  *health.HTTPHealthChecker
	statsCollector ports.StatsCollector
	statsService   *StatsService
	logger         logger.StyledLogger
	modelDiscovery *discovery.ModelDiscoveryService
	registry       domain.ModelRegistry
	endpointRepo   domain.EndpointRepository
	// purgeDeadFn, when set, is called with the current routable endpoint list
	// whenever a backend transitions to unhealthy. Wired by the proxy layer so sticky
	// session entries for dead backends are evicted promptly rather than waiting for TTL.
	// Stored atomically because SetPurgeDeadEndpointsFn is called from the main goroutine
	// while the health-check goroutine may concurrently invoke the callback.
	purgeDeadFn atomic.Pointer[func([]*domain.Endpoint)]
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(
	config *config.DiscoveryConfig,
	registryConfig *config.ModelRegistryConfig,
	statsCollector ports.StatsCollector,
	logger logger.StyledLogger,
) *DiscoveryService {
	return &DiscoveryService{
		config:         config,
		registryConfig: registryConfig,
		statsCollector: statsCollector,
		logger:         logger,
	}
}

// Name returns the service name
func (s *DiscoveryService) Name() string {
	return "discovery"
}

// Start initialises discovery components
func (s *DiscoveryService) Start(ctx context.Context) error {
	s.logger.Info("Initialising discovery service")

	if s.statsService != nil {
		collector, err := s.statsService.GetCollector()
		if err != nil {
			return fmt.Errorf("failed to get stats collector: %w", err)
		}
		s.statsCollector = collector
	}

	// Create model registry using factory with configuration
	if s.registryConfig == nil {
		// Fallback to default if no config provided
