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
