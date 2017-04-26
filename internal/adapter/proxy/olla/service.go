package olla

// 											Olla Proxy Implementation
//
// The Olla proxy implementation is a high-performance, resilient reverse proxy purpose-built for AI inference traffic
// (eg. LLMs, embedding APIs). It improves on Sherpa's implementation with additional safeguards, tuning and zero-GC
// optimisations. Most of the code is inspired by Sherpa, but Olla introduces several enhancements.
//
// Compared to Sherpa, Olla introduces:
// - **Per-endpoint connection pools**: Enables isolated TCP connection reuse, avoiding cross-endpoint interference.
// - **Circuit breakers**: Automatically trips on failure patterns to prevent cascading errors and allow graceful recovery.
// - **Aggressive object pooling**: Reuses request contexts, buffers and error objects to minimise heap allocations and GC pauses.
// - **Atomic stats correction**: Tracks min/max/total latencies lock-free under high concurrency.
// - **TCP optimisations**: Fine-grained tuning (eg. `SetNoDelay`, long keep-alive) designed for streaming workloads.
// - **Backpressure safe streaming**: Handles partial reads, client disconnects and stalled upstreams with resilient fallbacks.
//
// Suitable for workloads with:
// - Long-lived, token-streaming HTTP responses
// - Intermittently unreliable clients (eg. mobile devices, mini-PCs)
// - Multiple backend replicas (with health-state divergence)
//
// Olla is designed for edge/gateway use cases requiring robustness, high availability and minimal jitter under load.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/puzpuzpuz/xsync/v4"

	"github.com/putrkobey/Olla/internal/adapter/health"
	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/adapter/resilience"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/pkg/pool"
)

const (
	// Olla-specific constants (others are in proxy package)
	DefaultTLSHandshakeTimeout = 10 * time.Second
	DefaultSetNoDelay          = true

	ClientDisconnectionBytesThreshold = 1024
	ClientDisconnectionTimeThreshold  = 5 * time.Second

	// Circuit breaker threshold higher than health checker for tolerance
	circuitBreakerThreshold = 5 // vs health.DefaultCircuitBreakerThreshold (3)

	// HalfOpenStaleness bounds how long a single in-flight half-open probe can
	// gate out further probes before the slot is handed to a replacement caller.
	// This governs proxied inference requests, not health probes: TTFT alone
	// routinely exceeds a second on a cold or busy backend, and engine-side
	// timeouts run to minutes, so a sub-second window (as used by the health
	// checker's own sub-second /health probes) would misclassify a healthy
	// streaming probe as hung. 30s is sized to the same order of magnitude as
	// ResponseHeaderTimeout - long enough that a normal inference response
	// header arrives well within it, short enough that a truly hung probe
	// doesn't wedge the endpoint for long. Deliberately not copied from
	// health.CircuitBreaker, whose constant answers a different question.
	HalfOpenStaleness = 30 * time.Second
)

// Service implements the Olla proxy - optimised for high performance and resilience
type Service struct {
	*core.BaseProxyComponents

	// Buffer pool for streaming
	bufferPool *pool.Pool[*[]byte]

	transport     *http.Transport
	configuration atomic.Pointer[Configuration]
	retryHandler  *core.RetryHandler

	cleanupTicker *time.Ticker
	cleanupStop   chan struct{}

	// Per-endpoint connection pools and circuit breakers
	endpointPools   xsync.Map[string, *connectionPool]
	circuitBreakers xsync.Map[string, *circuitBreaker]

	// Cleanup management
	cleanupOnce sync.Once
}

// connectionPool isolates HTTP transport instances per endpoint
type connectionPool struct {
	transport *http.Transport
	lastUsed  int64 // atomic
	healthy   int64 // atomic: 0=unhealthy, 1=healthy
}

// circuitBreaker is a local alias for the shared closed/open/half-open state
// machine in internal/adapter/resilience - kept as a distinct name so this
// package's call sites and tests don't need renaming. See GetCircuitBreaker
// for the threshold/openDuration this package configures it with, and the
// comment above IsOpen's old home (now on resilience.Breaker) for why a
// second, independent copy of this state machine existed here at all.
type circuitBreaker = resilience.Breaker

// NewService creates a new Olla proxy service
func NewService(
	discoveryService ports.DiscoveryService,
	selector domain.EndpointSelector,
	configuration *Configuration,
	statsCollector ports.StatsCollector,
	metricsExtractor ports.MetricsExtractor,
	logger logger.StyledLogger,
) (*Service, error) {

	if configuration.StreamBufferSize == 0 {
		configuration.StreamBufferSize = proxyconfig.OllaDefaultStreamBufferSize
	}
	if configuration.MaxIdleConns == 0 {
		configuration.MaxIdleConns = proxyconfig.OllaDefaultMaxIdleConns
	}
	if configuration.MaxConnsPerHost == 0 {
		configuration.MaxConnsPerHost = proxyconfig.OllaDefaultMaxConnsPerHost
	}
	if configuration.MaxIdleConnsPerHost == 0 {
		configuration.MaxIdleConnsPerHost = proxyconfig.OllaDefaultMaxIdleConnsPerHost
	}
	if configuration.IdleConnTimeout == 0 {
		configuration.IdleConnTimeout = proxyconfig.OllaDefaultIdleConnTimeout
	}
	if configuration.ReadTimeout == 0 {
		configuration.ReadTimeout = proxyconfig.OllaDefaultReadTimeout
	}

	base := core.NewBaseProxyComponents(discoveryService, selector, statsCollector, metricsExtractor, logger)

	bufferPool, err := pool.NewLitePool(func() *[]byte {
		buf := make([]byte, configuration.StreamBufferSize)
		return &buf
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create buffer pool: %w", err)
	}

	transport := createOptimisedTransport(configuration)

	service := &Service{
		BaseProxyComponents: base,
		bufferPool:          bufferPool,
		transport:           transport,
		retryHandler:        core.NewRetryHandler(discoveryService, logger, core.WithRetryPolicy(configuration.GetRetryPolicy())),
		circuitBreakers:     *xsync.NewMap[string, *circuitBreaker](),
		endpointPools:       *xsync.NewMap[string, *connectionPool](),
		cleanupTicker:       time.NewTicker(5 * time.Minute),
		cleanupStop:         make(chan struct{}),
	}
	service.configuration.Store(configuration)

	// Start cleanup goroutine
	go service.cleanupLoop()

	return service, nil
}

// createOptimisedTransport creates an HTTP transport optimised for AI workloads
func createOptimisedTransport(config *Configuration) *http.Transport {
	return &http.Transport{
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.GetTLSHandshakeTimeout(),
		DisableCompression:    true,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: config.GetResponseHeaderTimeout(),
		// Olla targets local inference backends; outbound proxy env vars are not
		// honoured here because they would route credentialled requests through an
		// intermediary on plain HTTP. The health checker and model discovery follow
		// the same policy (adapter/factory.NewBackendHTTPClient).
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   config.GetConnectionTimeout(),
				KeepAlive: config.GetConnectionKeepAlive(),
			}
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				// We ignore errors for these settings on purpose
				_ = tcpConn.SetNoDelay(DefaultSetNoDelay)
				_ = tcpConn.SetKeepAlive(true)
				_ = tcpConn.SetKeepAlivePeriod(config.GetConnectionKeepAlive())
			}
			return conn, nil
		},
		MaxResponseHeaderBytes: 32 << 10, // 32KB
		WriteBufferSize:        64 << 10, // 64KB
		ReadBufferSize:         64 << 10, // 64KB
	}
}

// Configuration returns the effective, fully-defaulted configuration currently in
// use. Exposed for diagnostics and for tests that verify factory-supplied defaults
// actually reach the engine rather than getting overwritten upstream.
func (s *Service) Configuration() *Configuration {
	return s.configuration.Load()
}

// getOrCreateEndpointPool returns a connection pool for the endpoint.
// LoadOrCompute guarantees the transport is constructed at most once per endpoint key,
// preventing wasted allocations when multiple goroutines race on first use.
func (s *Service) getOrCreateEndpointPool(endpoint string) *connectionPool {
	cfg := s.configuration.Load()
	pool, _ := s.endpointPools.LoadOrCompute(endpoint, func() (*connectionPool, bool) {
		return &connectionPool{
			transport: createOptimisedTransport(cfg),
			lastUsed:  time.Now().UnixNano(),
			healthy:   1,
		}, false
	})
	atomic.StoreInt64(&pool.lastUsed, time.Now().UnixNano())
	return pool
}

// GetCircuitBreaker returns the circuit breaker for an endpoint (exported for testing).
// LoadOrCompute guarantees exactly one circuitBreaker is created per endpoint even
// under concurrent first-use, avoiding a redundant allocation race.
func (s *Service) GetCircuitBreaker(endpoint string) *circuitBreaker {
	cb, _ := s.circuitBreakers.LoadOrCompute(endpoint, func() (*circuitBreaker, bool) {
		return resilience.New(resilience.Config{
			FailureThreshold: circuitBreakerThreshold,
			OpenDuration:     health.DefaultCircuitBreakerTimeout,
		}), false
	})
	return cb
}

// Circuit breaker usage
//
// This is the second of two layers of failure protection, and it rarely
// gets the chance to open on the most common failure class: a
// connection-refused error is handled one layer up, in
// core.RetryHandler.handleConnectionFailure, which demotes the endpoint to
// offline via the health-check path after a single failure - well before
// this breaker's 5-failure threshold could accumulate. That demotion is
// intentional layering, not something that makes this breaker redundant: it
// governs a different failure class. A backend that accepts the TCP
// connection but then hangs or blows past ResponseHeaderTimeout never hits
// handleConnectionFailure at all (the connection succeeded), so this
// breaker is what actually protects against that class in practice -
// hangs and slow/unresponsive backends, not down ones.
//
// The state machine itself (closed/open/half-open, single-flight probe
// gating, stale-handover, ABA-safe attempt tokens) lives in
// resilience.Breaker, shared with adapter/health's circuit breaker - see
// that package for the mechanics. Call sites here pass HalfOpenStaleness
// (see its doc comment above) as the probe-staleness window on every
// IsOpen call, since that value is fixed per endpoint in this package
// rather than derived per call the way adapter/health derives it from
// CheckTimeout.

// ProxyRequest handles incoming HTTP requests
func (s *Service) ProxyRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, stats *ports.RequestStats, rlog logger.StyledLogger) error {
	endpoints, err := s.DiscoveryService.GetHealthyEndpoints(ctx)
	if err != nil {
		return err
	}

	return s.ProxyRequestToEndpoints(ctx, w, r, endpoints, stats, rlog)
}

// ProxyRequestToEndpoints delegates to retry-aware implementation
func (s *Service) ProxyRequestToEndpoints(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
	return s.ProxyRequestToEndpointsWithRetry(ctx, w, r, endpoints, stats, rlog)
}

// prepareProxyRequest creates and prepares the proxy request with headers.
// endpoint is passed through so CopyHeaders can apply per-endpoint auth and custom headers.
func (s *Service) prepareProxyRequest(ctx context.Context, r *http.Request, targetURL *url.URL, endpoint *domain.Endpoint, stats *ports.RequestStats) (*http.Request, error) {
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL.String(), r.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	headerStart := time.Now()
	core.CopyHeaders(proxyReq, r, endpoint)
	stats.HeaderProcessingMs = time.Since(headerStart).Milliseconds()

	// Add model header
	if model, ok := ctx.Value(constants.ContextModelKey).(string); ok && model != "" {
		proxyReq.Header.Set(constants.HeaderXModel, model)
		stats.Model = model
	}

	// Mark request processing complete
	stats.RequestProcessingMs = time.Since(stats.StartTime).Milliseconds()
