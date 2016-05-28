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

	s.logger.Info("HTTP server listening",
		"address", addr,
		"readTimeout", readTimeout,
		"readHeaderTimeout", readHeaderTimeout,
		"writeTimeout", writeTimeout,
		"idleTimeout", idleTimeout)

	go func() {
		if serr := s.server.Serve(ln); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			s.logger.Error("HTTP server error", "error", serr)
		}
	}()

	s.logger.Info("Olla started, waiting for requests...", "bind", addr)

	s.printWarnings()
	s.printDashboardURL(addr)
	return nil
}

// printDashboardURL logs a standout line pointing at the admin dashboard once
// the server is otherwise fully up. It runs after printWarnings so it is the
// last thing an operator sees at the bottom of the startup log, mirroring the
// InfoWithStatus "[ OK ]" treatment Stop() already uses for its loudest line.
// Skipped silently if the bind address can't be turned into a URL - this is
// cosmetic, not worth failing startup over.
//
// The line is honest about asset presence: a binary built without `make
// build-web` (e.g. via `go install`) carries only the .gitkeep sentinel, so
// the dashboard route would 503 on every request. Claiming "ready" there
// sends the operator on a dead debugging loop. We log a separate line that
// names the fix instead.
func (s *HTTPService) printDashboardURL(bindAddr string) {
	if !s.fullConfig.Dashboard.Enabled {
		return
	}

	url, ok := dashboardURL(bindAddr)
	if !ok {
		return
	}

	s.logger.Info(dashboardStartupLine(url, dashboard.AssetsBuilt()))
}

// dashboardStartupLine renders the line printDashboardURL emits for a given
// URL and asset-present flag. Split out so the decision is unit-testable
// without standing up a logger.
func dashboardStartupLine(url string, assetsBuilt bool) string {
	if assetsBuilt {
		return fmt.Sprintf("Admin dashboard ready - %s", url)
	}
	return fmt.Sprintf("Admin dashboard at %s (assets not built - run `make build-web` or use a release binary)", url)
}

// dashboardURL derives a clickable dashboard URL from a listener bind address.
// An unspecified host (0.0.0.0, ::, [::], or an empty host) isn't something a
// browser can open, so it's substituted with localhost. IPv6 literals are
// bracketed for a valid URL authority.
func dashboardURL(bindAddr string) (string, bool) {
	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil || port == "" {
		return "", false
	}

	// net.JoinHostPort brackets IPv6 literals itself, so displayHost is left
	// unbracketed here regardless of address family.
	displayHost := host
	if addr, perr := netip.ParseAddr(host); host == "" || (perr == nil && addr.IsUnspecified()) {
		displayHost = "localhost"
	}

	return fmt.Sprintf("http://%s/internal/ui/", net.JoinHostPort(displayHost, port)), true
}

// applyCORS wraps the root handler with CORS as the outermost layer so that
// browser preflight (OPTIONS) requests are answered directly by rs/cors and
// never reach the per-route security chain (rate-limiting, access logging).
// This is correct CORS behaviour: preflights carry no credentials or body and
// never reach a backend, so letting the security chain return 401/403/429 to
// a preflight probe would break legitimate browser clients. Actual GET/POST
// requests pass through rs/cors and then traverse the full security chain as
// normal. Kept as a seam so the enable/disable wiring is testable without
// standing up the full HTTP server.
func applyCORS(handler http.Handler, cfg config.CorsConfig) http.Handler {
	if !cfg.Enabled {
		return handler
	}
	return middleware.NewCORS(cfg).Handler(handler)
}

// bindListener binds a TCP listener on addr synchronously so that any
// port-in-use error surfaces immediately to the caller. Using ListenConfig
// means the listener also respects context cancellation during startup.
func bindListener(ctx context.Context, addr string) (net.Listener, error) {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
