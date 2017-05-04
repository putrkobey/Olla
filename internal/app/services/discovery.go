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
		s.registry = registry.NewMemoryModelRegistry(s.logger)
	} else {
		registryConfig := registry.RegistryConfig{
			Type:            s.registryConfig.Type,
			EnableUnifier:   s.registryConfig.EnableUnifier,
			UnificationConf: &s.registryConfig.Unification,
			// Routing strategy must be forwarded so the configured type (strict,
			// optimistic, discovery) takes effect. Without this, registries fall
			// back to strict regardless of what the operator configured.
			RoutingStrategy: &s.registryConfig.RoutingStrategy,
		}
		var err error
		s.registry, err = registry.NewModelRegistry(registryConfig, s.logger)
		if err != nil {
			return fmt.Errorf("failed to create model registry: %w", err)
		}
	}

	switch s.config.Type {
	case "static":
		staticRepo := discovery.NewStaticEndpointRepository()
		if err := staticRepo.LoadFromConfig(ctx, s.config.Static.Endpoints); err != nil {
			return fmt.Errorf("failed to load endpoints from config: %w", err)
		}
		s.endpointRepo = staticRepo
	default:
		return fmt.Errorf("unsupported discovery type: %s", s.config.Type)
	}

	s.healthChecker = health.NewHTTPHealthCheckerWithDefaults(s.endpointRepo, s.logger)

	// Purge sticky session entries for any backend that goes offline. The purgeFn is
	// nil when sticky sessions are disabled, making this callback a cheap no-op then.
	s.healthChecker.SetUnhealthyCallback(health.UnhealthyCallbackFunc(func(ctx context.Context, _ *domain.Endpoint) {
		fn := s.purgeDeadFn.Load()
		if fn == nil {
			return
		}
		routable, err := s.endpointRepo.GetRoutable(ctx)
		if err != nil {
			s.logger.Warn("Failed to fetch routable endpoints for sticky session purge", "error", err)
			return
		}
		(*fn)(routable)
	}))

	if err := s.healthChecker.StartChecking(ctx); err != nil {
		return fmt.Errorf("failed to start health checker: %w", err)
	}

	// Critical: Run initial health check to validate endpoints immediately.
	// This prevents the proxy from starting with no healthy endpoints.
	if err := s.healthChecker.RunHealthCheck(ctx, true); err != nil {
		s.logger.Warn("Failed to force initial health check", "error", err)
	}

	endpoints, err := s.endpointRepo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get endpoints: %w", err)
	}

	for _, endpoint := range endpoints {
		s.logger.Info("Endpoint registered",
			"name", endpoint.Name,
			"url", endpoint.URLString,
			"priority", endpoint.Priority)
	}

	if s.config.ModelDiscovery.Enabled {
		httpClient := newDiscoveryHTTPClient(s.config.ModelDiscovery.Timeout)
		profileFactory, err := profile.NewFactoryWithDefaults()
		if err != nil {
			return fmt.Errorf("failed to create profile factory: %w", err)
		}
		client := discovery.NewHTTPModelDiscoveryClient(profileFactory, s.logger, httpClient)
		discoveryConfig := discovery.DiscoveryConfig{
			Interval:          s.config.ModelDiscovery.Interval,
			Timeout:           s.config.ModelDiscovery.Timeout,
