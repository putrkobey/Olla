package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/metrics"
	"github.com/putrkobey/Olla/internal/adapter/proxy"
	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	// defaultConnectionKeepAlive is the fallback when the proxy config omits
	// connection_keep_alive. Matches the app-layer default to keep behaviour
	// identical for operators who haven't set the field.
	defaultConnectionKeepAlive = 30 * time.Second
)

// ProxyServiceWrapper adapts the core proxy implementation to the service lifecycle
// model. It manages the creation of load balancers and proxy engines, ensuring they
// receive validated endpoints from the discovery service.
type ProxyServiceWrapper struct {
	config       *config.ProxyConfig
	proxyService ports.ProxyService
	loadBalancer domain.EndpointSelector
	// stickyWrapper is non-nil when sticky sessions are enabled; held separately
	// so Stop() can shut down its background goroutine.
	stickyWrapper    *balancer.StickySessionWrapper
	endpointRepo     domain.EndpointRepository
	discoveryService ports.DiscoveryService
	statsCollector   ports.StatsCollector
	metricsExtractor ports.MetricsExtractor
	statsService     *StatsService
	discoverySvc     *DiscoveryService
	securityService  *SecurityService
	logger           logger.StyledLogger
}

// NewProxyServiceWrapper creates a new proxy service wrapper
func NewProxyServiceWrapper(
	config *config.ProxyConfig,
	logger logger.StyledLogger,
) *ProxyServiceWrapper {
	return &ProxyServiceWrapper{
		config: config,
		logger: logger,
	}
}

// Name returns the service name
func (s *ProxyServiceWrapper) Name() string {
	return "proxy"
}

// Start initialises the proxy service and load balancer
func (s *ProxyServiceWrapper) Start(ctx context.Context) error {
	s.logger.Info("Initialising proxy service")

	// Get dependencies from services
	if s.statsService != nil {
		collector, err := s.statsService.GetCollector()
		if err != nil {
			return fmt.Errorf("failed to get stats collector: %w", err)
		}
		s.statsCollector = collector
	}
	if s.discoverySvc != nil {
		repo, err := s.discoverySvc.GetEndpointRepository()
		if err != nil {
			return fmt.Errorf("failed to get endpoint repository: %w", err)
		}
		s.endpointRepo = repo

		discService, err := s.discoverySvc.GetDiscoveryService()
		if err != nil {
			return fmt.Errorf("failed to get discovery service: %w", err)
		}
		s.discoveryService = discService
	}

	// Create load balancer
	balancerFactory := balancer.NewFactory(s.statsCollector)
	var err error
	s.loadBalancer, err = balancerFactory.Create(s.config.LoadBalancer)
	if err != nil {
		return fmt.Errorf("failed to create load balancer: %w", err)
	}
	s.logger.Info("Load balancer created", "type", s.config.LoadBalancer)

	s.applyStickySessions()

	// Create proxy configuration
	proxyConfig := s.createProxyConfiguration()

	// Create a discovery service adapter
	s.discoveryService = &endpointRepositoryAdapter{repo: s.endpointRepo}

	// Create metrics extractor with profile factory
	s.logger.Info("Creating metrics extractor...")
	profileFactory, err := profile.NewFactoryWithDefaults()
	if err != nil {
		s.logger.Warn("Failed to create profile factory for metrics extraction", "error", err)
		// Continue without metrics extraction
		profileFactory = nil
	} else {
		s.logger.Info("Profile factory created successfully",
			"profiles", profileFactory.GetAvailableProfiles())
	}

	var metricsExtractor ports.MetricsExtractor
	if profileFactory != nil {
		metricsExtractor, err = metrics.NewExtractor(profileFactory, s.logger)
		if err != nil {
			s.logger.Warn("Failed to create metrics extractor", "error", err)
			// Continue without metrics extraction
			metricsExtractor = nil
		} else {
			s.logger.Info("Metrics extractor initialized successfully")
