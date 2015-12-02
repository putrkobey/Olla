package health

import (
	"errors"
	"net/http"
	"time"

	"github.com/puzpuzpuz/xsync/v4"

	"github.com/putrkobey/Olla/internal/adapter/resilience"
)

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
)

// HTTPClient interface for better testability
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// CircuitBreaker tracks failure rates and prevents cascading failures. It is
// a thin, endpoint-keyed adapter around resilience.Breaker - the shared
// closed/open/half-open state machine also used by proxy/olla's circuit
// breaker - adding only what is specific to health checks: one Breaker per
// health-check URL, and a per-endpoint half-open staleness window derived
// from that endpoint's CheckTimeout (see probeStaleness).
// TODO: (HOT-RELOAD) Add cleanup mechanism for removed endpoints when hot reload is implemented
// The endpoints xsync.Map will accumulate stale entries for removed/changed endpoints without TTL
type CircuitBreaker struct {
	endpoints        *xsync.Map[string, *resilience.Breaker]
	failureThreshold int
	timeout          time.Duration
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		endpoints:        xsync.NewMap[string, *resilience.Breaker](),
		failureThreshold: DefaultCircuitBreakerThreshold,
		timeout:          DefaultCircuitBreakerTimeout,
	}
}

// probeStaleness derives the half-open single-flight staleness window from
// the endpoint's own CheckTimeout, rather than a single fixed constant: this
// CircuitBreaker is shared across every endpoint (one xsync.Map keyed by
// health-check URL), but CheckTimeout is per-endpoint config, so the window
// has to be computed per call from whatever the caller passes in, not baked
// into the CircuitBreaker at construction.
//
// checker.go's Check wraps each attempt - including HealthClient's own
// internal retries - in a context timeout of CheckTimeout*2, so a
// legitimately slow-but-alive probe can genuinely take that long to
// resolve. +1s margin absorbs scheduling jitter right around that boundary.
// A fixed 1s window (this package's original value) was tighter than that
// real probe budget and could trigger a spurious stale-handover - admitting
// a second concurrent probe - against a backend that was merely slow, not
// hung.
//
// checkTimeout <= 0 (a caller with no real endpoint, e.g. a test) falls back
// to defaultCheckTimeoutFallback, matching config.go's own endpoint default,
// so the window is never zero.
func probeStaleness(checkTimeout time.Duration) time.Duration {
	if checkTimeout <= 0 {
		checkTimeout = defaultCheckTimeoutFallback
	}
	return checkTimeout*2 + time.Second
}

// IsOpen reports whether the circuit is open for endpointURL, given that
// endpoint's own CheckTimeout (used to derive the half-open staleness
