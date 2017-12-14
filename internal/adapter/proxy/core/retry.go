package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// RetryHandler manages connection failure recovery and endpoint failover
type RetryHandler struct {
	logger           logger.StyledLogger
	discoveryService ports.DiscoveryService
	policy           proxyconfig.RetryPolicy
}

// RetryOption customises a RetryHandler at construction.
type RetryOption func(*RetryHandler)

// WithRetryPolicy sets the failover policy. Without it the handler fails over
// to every eligible endpoint once and replays any method after a connection failure.
func WithRetryPolicy(p proxyconfig.RetryPolicy) RetryOption {
	return func(h *RetryHandler) { h.policy = p }
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(discoveryService ports.DiscoveryService, logger logger.StyledLogger, opts ...RetryOption) *RetryHandler {
	h := &RetryHandler{
		discoveryService: discoveryService,
		logger:           logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// maxAttempts is the per-request attempt budget for the given candidate count.
func (h *RetryHandler) maxAttempts(candidates int) int {
	switch {
	case h.policy.Disabled:
		return 1
	case h.policy.MaxAttempts > 0:
		return min(candidates, h.policy.MaxAttempts)
	default:
		return candidates
	}
}

// ProxyFunc defines the signature for endpoint proxy implementations
type ProxyFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoint *domain.Endpoint, stats *ports.RequestStats) error

// ResponseStartedWriter wraps http.ResponseWriter and records whether any response
// bytes or status codes have been sent to the client. Retry logic uses this to gate
// retry decisions: once the client has received data, retrying a non-idempotent
// request would send duplicate content or charge twice on metered APIs. It is also
// the correct "has this response been committed" signal for callers upstream of
// retry (e.g. handler_proxy.go's handleProxyError) - Header().Set does not commit
// anything, only WriteHeader/Write do, so a Content-Type-presence check is not a
// reliable substitute.
//
// A flush is also a commit: streaming code can in principle reach a bare Flush()
// (via http.NewResponseController) without a prior Write on this wrapper - bytes
// already buffered by the underlying writer would then reach the client while
// started stays false. Flush/FlushError guard against that by marking started
// before delegating, so the signal holds regardless of call order.
type ResponseStartedWriter struct {
	http.ResponseWriter
	started bool
	status  int
}

// NewResponseStartedWriter wraps w so callers can observe whether a response has
// been committed via Started().
func NewResponseStartedWriter(w http.ResponseWriter) *ResponseStartedWriter {
	return &ResponseStartedWriter{ResponseWriter: w}
}

func (rw *ResponseStartedWriter) WriteHeader(code int) {
	rw.started = true
	// 1xx interim responses are not the final status the client sees.
	if rw.status == 0 && code >= http.StatusOK {
		rw.status = code
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseStartedWriter) Write(b []byte) (int, error) {
	rw.started = true
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// Started reports whether a status code or body bytes have been committed to the
// client yet.
func (rw *ResponseStartedWriter) Started() bool {
	return rw.started
}

// Status is the final status code sent to the client: 0 until a response is committed,
// and 200 when body bytes were written without an explicit WriteHeader.
func (rw *ResponseStartedWriter) Status() int {
	return rw.status
}

// Unwrap exposes the underlying ResponseWriter so http.NewResponseController can
// discover optional interfaces (Flush, Hijack, SetDeadline, etc.) via the chain.
// Without this, the wrapper hides the underlying flusher and SSE streams stall.
func (rw *ResponseStartedWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// FlushError marks the response as started and flushes the underlying writer,
// mirroring http.ResponseController's FlushError semantics. It returns
// http.ErrNotSupported if nothing in the underlying chain supports flushing,
// rather than panicking or swallowing the failure.
//
// started is set before the flush is attempted: a flush pushes any buffered
// bytes to the client immediately, so the commit has already happened by the
// time this call is made, even if the underlying flush itself then fails.
func (rw *ResponseStartedWriter) FlushError() error {
	wasStarted, wasStatus := rw.started, rw.status
	rw.started = true
	// A bare flush commits an implicit 200, same as a body write.
	if rw.status == 0 {
		rw.status = http.StatusOK
	}

	rc := http.NewResponseController(rw.ResponseWriter)
	if err := rc.Flush(); err != nil {
		// ErrNotSupported means nothing in the chain could flush at all, so
		// nothing was committed - restore the prior state. Any other error
		// (dead connection, etc.) may have partially written bytes before
		// failing, so started stays true.
		if errors.Is(err, http.ErrNotSupported) {
			rw.started, rw.status = wasStarted, wasStatus
		}
		return err
	}
	return nil
}

// Flush satisfies the plain http.Flusher interface for callers that still
// type-assert for it directly. Like http.Flusher.Flush, it is best-effort and
// discards any error - use FlushError to observe failures.
func (rw *ResponseStartedWriter) Flush() {
	_ = rw.FlushError()
}

// isIdempotent reports whether the HTTP method is safe to replay on another endpoint
// after a connection failure. GET, HEAD and OPTIONS are defined as idempotent by
// RFC 9110; POST, PATCH and DELETE are not. Replaying them risks double-billing or
// duplicate side effects if the first backend already read and acted on the request.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// ExecuteWithRetry attempts request delivery with automatic failover on connection errors.
// Retry is always suppressed once any part of the response has reached the client,
// whatever the method: a second endpoint's reply would be appended to the first's
// partial output. The attempt budget and whether non-idempotent methods may be
// replayed after a connection failure come from the handler's RetryPolicy.
func (h *RetryHandler) ExecuteWithRetry(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	endpoints []*domain.Endpoint,
	selector domain.EndpointSelector,
	stats *ports.RequestStats,
	proxyFunc ProxyFunc,
) error {
	if len(endpoints) == 0 {
		return errors.New("no endpoints available")
	}

	// Work with a copy to avoid modifying the original slice
	availableEndpoints := make([]*domain.Endpoint, len(endpoints))
	copy(availableEndpoints, endpoints)

	// Preserve request body for potential retries
	bodyBytes, err := h.preserveRequestBody(r)
	if err != nil {
		return err
	}

	// Wrap the writer so we can detect when bytes have been committed to the client.
	tracker := NewResponseStartedWriter(w)

	var lastErr error
	maxRetries := h.maxAttempts(len(endpoints))
	// attemptCount is the retry budget: only attempts that reached a backend count.
	// iteration counts every pass, so the body is re-armed after any earlier one.
	attemptCount, iteration := 0, 0
	// Tracked separately from lastErr because lastErr only ever reflects the
	// final attempt: if earlier endpoints failed with real connection errors
	// and the final one was CB-open, lastErr alone would mislabel the whole
	// exhaustion as "circuit breaker open" and hide the network failures.
	var cbOpenCount, connErrCount int

	for attemptCount < maxRetries && len(availableEndpoints) > 0 {
		if err := h.checkContextCancellation(ctx); err != nil {
			return err
		}

		h.resetRequestBodyForRetry(r, bodyBytes, iteration)
		iteration++

		endpoint, err := selector.Select(ctx, availableEndpoints)
		if err != nil {
			return fmt.Errorf("endpoint selection failed: %w", err)
		}

		attemptCount++
		lastErr = h.executeProxyAttempt(ctx, tracker, r, endpoint, selector, stats, proxyFunc)

		if lastErr == nil {
			return nil
		}

		// A CB-open rejection is failover-eligible, but unlike a connection
		// error it must NOT demote the endpoint's persisted health: the breaker
		// already reflects accumulated failure state, and the proxy engine
		// owns demotion for genuine connectivity failures. Remove the endpoint
		// from this request's candidate list only, then continue. Evaluated
		// before IsConnectionError so the sentinel never reaches the
		// connection-error/handleConnectionFailure path below.
		if errors.Is(lastErr, ErrCircuitBreakerOpen) {
			cbOpenCount++
			// The endpoint was never contacted, so the rejection does not spend
			// the attempt budget: with retry disabled or a small max_attempts a
			// healthy peer must still be reachable. The loop still terminates
			// because the endpoint leaves the candidate list below.
			attemptCount--
			// Once the response has started we cannot safely retry even on
			// CB-open: the same partial-response hazard applies. The guard runs
			// identically on every failover path.
			if tracker.started {
				h.logger.Debug("skipping retry: response already started",
					"method", r.Method,
					"endpoint", endpoint.Name)
				return lastErr
			}
			h.logger.Debug("circuit breaker open, failing over to next endpoint",
				"endpoint", endpoint.Name,
				"remaining_endpoints", len(availableEndpoints)-1)
			availableEndpoints = h.removeFailedEndpoint(availableEndpoints, endpoint)
			continue
		}

		if !IsConnectionError(lastErr) {
			// Non-connection error warrants immediate failure
			return lastErr
		}
		connErrCount++

		// Once the response has started we cannot safely retry, whatever the
		// method. The client would receive a partial response from this
