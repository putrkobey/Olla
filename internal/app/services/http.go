package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/putrkobey/Olla/internal/app/handlers"
	"github.com/putrkobey/Olla/internal/app/handlers/dashboard"
	"github.com/putrkobey/Olla/internal/app/middleware"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	// defaultReadHeaderTimeout protects the inbound server against Slowloris-style
	// attacks. A legitimate client sends its full header set well within 10 s; slow
	// backends are handled by proxy.ConnectionTimeout which applies much later.
	defaultReadHeaderTimeout = 10 * time.Second

	// minMaxHeaderBytes floors the derived MaxHeaderBytes. A configured value of
	// zero or negative must not be read as "disable the cap" (Go's http.Server
	// treats <=0 as "use the 1 MiB default", which is a reasonable fallback, but
	// a small positive misconfiguration deserves a sane floor rather than
	// crippling every request with oversized-but-legitimate headers).
	minMaxHeaderBytes = 4 * 1024
)

// HTTPService manages the HTTP server lifecycle and route registration. It coordinates
// with other services to ensure the server only starts accepting requests after all
// dependencies are initialised and health checks have completed.
type HTTPService struct {
	config           *config.ServerConfig
	fullConfig       *config.Config
	server           *http.Server
	proxyService     ports.ProxyService
	statsCollector   ports.StatsCollector
	modelRegistry    domain.ModelRegistry
	securityChain    *ports.SecurityChain
	logger           logger.StyledLogger
	application      *handlers.Application
	discoveryService ports.DiscoveryService
	repository       domain.EndpointRepository
	statsService     *StatsService
	proxySvc         *ProxyServiceWrapper
	discoverySvc     *DiscoveryService
	securitySvc      *SecurityService
}

// NewHTTPService creates a new HTTP service
func NewHTTPService(
	config *config.ServerConfig,
	fullConfig *config.Config,
	logger logger.StyledLogger,
) *HTTPService {
	return &HTTPService{
		config:     config,
		fullConfig: fullConfig,
		logger:     logger,
	}
}

// Name returns the service name
func (s *HTTPService) Name() string {
	return "http"
}

// Start initialises and starts the HTTP server
func (s *HTTPService) Start(ctx context.Context) error {
	s.logger.Info("Initialising HTTP service")

	// Resolve service dependencies now that all services are started
	if s.statsService != nil {
		collector, err := s.statsService.GetCollector()
		if err != nil {
			return fmt.Errorf("failed to get stats collector: %w", err)
		}
		s.statsCollector = collector
	}
	if s.proxySvc != nil {
		proxyService, err := s.proxySvc.GetProxyService()
		if err != nil {
			return fmt.Errorf("failed to get proxy service: %w", err)
		}
		s.proxyService = proxyService
	}
	if s.discoverySvc != nil {
		registry, err := s.discoverySvc.GetRegistry()
		if err != nil {
			return fmt.Errorf("failed to get model registry: %w", err)
		}
		s.modelRegistry = registry

		discoveryService, err := s.discoverySvc.GetDiscoveryService()
		if err != nil {
			return fmt.Errorf("failed to get discovery service: %w", err)
		}
		s.discoveryService = discoveryService

		repository, err := s.discoverySvc.GetEndpointRepository()
		if err != nil {
			return fmt.Errorf("failed to get endpoint repository: %w", err)
		}
		s.repository = repository
	}
	if s.securitySvc != nil {
		chain, err := s.securitySvc.GetSecurityChain()
		if err != nil {
			return fmt.Errorf("failed to get security chain: %w", err)
		}
		s.securityChain = chain
	}

	readTimeout := s.config.ReadTimeout
	readHeaderTimeout := s.config.ReadHeaderTimeout
	if readHeaderTimeout == 0 {
		readHeaderTimeout = defaultReadHeaderTimeout
	}
	writeTimeout := s.config.WriteTimeout
	idleTimeout := s.config.IdleTimeout

	app, err := handlers.NewApplication(
		ctx,
		s.fullConfig,
		s.proxyService,
		s.statsCollector,
		s.modelRegistry,
		s.discoveryService,
		s.repository,
		s.securityChain,
		s.logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create application handler: %w", err)
	}
	s.application = app

	// Wire sticky session stats if enabled - proxySvc holds the wrapper.
	if s.proxySvc != nil {
		s.application.SetStickyStatsFn(s.proxySvc.StickyStats)
	}

	// Wire real security adapters so non-proxy routes get size validation.
	// This is separate from the security chain (which handles proxy routes) so
	// non-proxy routes are protected without requiring full chain enforcement.
	if s.securitySvc != nil {
		if adapters, adapterErr := s.securitySvc.GetAdapters(); adapterErr == nil && adapters != nil {
			s.application.SetSecurityAdapters(adapters)
		}
	}

	s.application.RegisterRoutes()

	// Wire routes with security middleware
	mux := http.NewServeMux()
	routeRegistry := s.application.GetRouteRegistry()
	securityAdapters := s.application.GetSecurityAdapters()
	routeRegistry.WireUpWithSecurityChain(mux, securityAdapters)

	var root http.Handler = mux
	root = applyCORS(root, s.fullConfig.Server.Cors)

	if s.fullConfig.Server.Cors.Enabled {
		s.logger.Info("CORS enabled",
			"allowed_origins", s.fullConfig.Server.Cors.AllowedOrigins,
			"allow_credentials", s.fullConfig.Server.Cors.AllowCredentials)
	}

	addr := s.config.GetAddress()

	ln, err := bindListener(ctx, addr)
	if err != nil {
		return err
	}

	s.server = &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytesFromConfig(s.fullConfig.Server.RequestLimits.MaxHeaderSize),
	}

