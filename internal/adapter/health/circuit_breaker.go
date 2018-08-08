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

