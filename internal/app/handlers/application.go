package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/adapter/security"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/app/middleware"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/router"
)

// SecurityAdapters provides middleware for security chain
type SecurityAdapters struct {
	securityChain    *ports.SecurityChain
	securityAdapters *security.Adapters // nil when security is not configured
	logger           logger.StyledLogger
	throttles        *logger.ThrottleSet // nil in tests; collects the request-log throttles

	// requestLog is built once and shared by every route: the route registry asks for the
	// chain per route, and a middleware built per route would hold its own rejection
	// throttle, so a client would get one sampled line per route per window rather than one.
	requestLog     func(http.Handler) http.Handler
	requestLogOnce sync.Once
}

func (s *SecurityAdapters) loggingMiddleware() func(http.Handler) http.Handler {
	s.requestLogOnce.Do(func() {
		s.requestLog = middleware.NewCombinedLoggingMiddleware(s.logger, s.throttles)
	})
	return s.requestLog
}

// CreateChainMiddleware creates middleware that applies the full security chain with enhanced logging.
// When concrete security adapters are available (production path), we delegate to them so that
// per-validator status codes (429 rate-limit, 413 body-too-large) are preserved. The abstract
// securityChain path is kept as a fallback for test contexts where only the chain is wired.
func (s *SecurityAdapters) CreateChainMiddleware(reject security.RejectFunc) func(http.Handler) http.Handler {
	return s.chain(reject, func(a *security.Adapters) func(http.Handler) http.Handler {
		return a.CreateChainMiddleware(reject)
	})
}

// CreateTranslatorChainMiddleware is CreateChainMiddleware for translator routes, which buffer
// their bodies and so take the buffering size variant. Rejections are written through reject
// in the translator's own error format; nil keeps the plain-text responses. bodyLimit is the
// translator's request cap, which bounds how much of a chunked body is buffered.
func (s *SecurityAdapters) CreateTranslatorChainMiddleware(reject security.RejectFunc, bodyLimit int64) func(http.Handler) http.Handler {
	return s.chain(reject, func(a *security.Adapters) func(http.Handler) http.Handler {
		return a.CreateTranslatorChainMiddleware(reject, bodyLimit)
	})
}

func (s *SecurityAdapters) chain(reject security.RejectFunc, concrete func(*security.Adapters) func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Wrap with logging so every proxy request is recorded regardless of which
		// security path runs below.
		withAccessLogging := s.loggingMiddleware()(next)

		if s.securityAdapters != nil {
			// Delegate to the concrete adapter chain. It sets the correct status codes
			// (429 + Retry-After/X-RateLimit-* for rate limiting, 413 for oversized
			// bodies) rather than flattening everything to 403.
			concreteChain := concrete(s.securityAdapters)(withAccessLogging)
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				concreteChain.ServeHTTP(w, r)
			})
		}

		// Fallback: abstract chain only (e.g. unit tests that inject securityChain
		// directly without wiring the full security.Adapters).
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.securityChain != nil {
				secReq := ports.SecurityRequest{
					ClientID:      r.RemoteAddr,
					Endpoint:      r.URL.Path,
					Method:        r.Method,
					BodySize:      r.ContentLength,
					HeaderSize:    0,
					Headers:       r.Header,
					IsHealthCheck: r.URL.Path == "/internal/health",
				}

				result, err := s.securityChain.Validate(r.Context(), secReq)
				if err != nil || !result.Allowed {
					if reject != nil {
						reject(w, http.StatusForbidden, "Security validation failed")
						return
					}
					http.Error(w, "Security validation failed", http.StatusForbidden)
					return
				}
			}
			withAccessLogging.ServeHTTP(w, r)
		})
	}
}

// CreateRateLimitMiddleware creates middleware that only applies rate limiting with enhanced logging
func (s *SecurityAdapters) CreateRateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Apply enhanced logging for non-proxy routes as well
		return s.loggingMiddleware()(next)
	}
}

// CreateSizeMiddleware returns a middleware that enforces request-size limits on
// non-proxy routes. Delegates to the real security adapter when available; returns
// a pass-through when security is not configured (e.g. in tests).
func (s *SecurityAdapters) CreateSizeMiddleware() func(http.Handler) http.Handler {
	if s.securityAdapters != nil {
		return s.securityAdapters.CreateSizeMiddleware()
	}
	return func(next http.Handler) http.Handler { return next }
}

// Application holds all the dependencies needed for the HTTP handlers
type Application struct {
	Config             *config.Config
	logger             logger.StyledLogger
	proxyService       ports.ProxyService
	statsCollector     ports.StatsCollector
	modelRegistry      domain.ModelRegistry
	discoveryService   ports.DiscoveryService
	repository         domain.EndpointRepository
	inspectorChain     *inspector.Chain
	pathInspector      *inspector.PathInspector
	securityAdapters   *SecurityAdapters
	routeRegistry      *router.RouteRegistry
	converterFactory   *converter.ConverterFactory
	profileFactory     profile.ProfileFactory
	profileLookup      translator.ProfileLookup
	translatorRegistry *translator.Registry
	// stickyStatsFn is non-nil when sticky sessions are enabled. Stored as a
	// closure so the handler layer does not need to import the balancer package.
	stickyStatsFn func() *balancer.StickyStats
	aliasResolver *registry.AliasResolver
	rejectLog     *rejectLogs
	// throttles holds every rejection-log throttle the application created, so shutdown
	// can report the tail of a flood that stopped just before it.
	throttles *logger.ThrottleSet
	server    *http.Server
	errCh     chan error
	StartTime time.Time
}

// NewApplication creates a new Application instance with all required dependencies
func NewApplication(
	ctx context.Context,
	cfg *config.Config,
	proxyService ports.ProxyService,
	statsCollector ports.StatsCollector,
	modelRegistry domain.ModelRegistry,
	discoveryService ports.DiscoveryService,
	repository domain.EndpointRepository,
	securityChain *ports.SecurityChain,
	logger logger.StyledLogger,
) (*Application, error) {
	// Create inspector chain
	profileFactory, err := profile.NewFactoryWithDefaults()
	if err != nil {
		// Try to create factory with empty profile dir (uses built-in profiles)
		profileFactory, err = profile.NewFactory("")
		if err != nil {
			logger.Error("Failed to create profile factory", "error", err)
			return nil, fmt.Errorf("cannot initialize profile factory: %w", err)
		}
		logger.Warn("Failed to load profile configurations, using built-in profiles", "error", err)
	}
	inspectorFactory := inspector.NewFactory(profileFactory, logger)
	inspectorChain := inspectorFactory.CreateChain()
	// Add path inspector
	pathInspector := inspectorFactory.CreatePathInspector()
	inspectorChain.AddInspector(pathInspector)
	// Add body inspector for model extraction
	bodyInspector, err := inspectorFactory.CreateBodyInspector()
	if err != nil {
		return nil, fmt.Errorf("failed to create body inspector: %w", err)
	}
