package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// ---- helpers ----------------------------------------------------------------

func newTestRetryHandler(t *testing.T) *RetryHandler {
	t.Helper()
	logCfg := &logger.Config{Level: "error"}
	log, _, _ := logger.New(logCfg)
	return NewRetryHandler(&testDiscoveryService{}, logger.NewPlainStyledLogger(log))
}

// roundRobinSelector cycles through endpoints in order.
type roundRobinSelector struct{ idx int }

func (s *roundRobinSelector) Select(_ context.Context, eps []*domain.Endpoint) (*domain.Endpoint, error) {
	if len(eps) == 0 {
		return nil, errors.New("no endpoints")
	}
	ep := eps[s.idx%len(eps)]
	s.idx++
	return ep, nil
}
func (s *roundRobinSelector) Name() string                            { return "round-robin" }
func (s *roundRobinSelector) IncrementConnections(_ *domain.Endpoint) {}
func (s *roundRobinSelector) DecrementConnections(_ *domain.Endpoint) {}

// namedEndpoint creates a minimal endpoint with the given name.
func namedEndpoint(name string) *domain.Endpoint {
	return &domain.Endpoint{Name: name, CheckTimeout: 0}
}

// connectionResetError satisfies net.Error so IsConnectionError returns true.
type connectionResetError struct{}

func (e *connectionResetError) Error() string   { return "connection reset by peer" }
func (e *connectionResetError) Timeout() bool   { return false }
func (e *connectionResetError) Temporary() bool { return false }

// Ensure it implements net.Error.
var _ net.Error = (*connectionResetError)(nil)

// ---- tests ------------------------------------------------------------------

// TestIsConnectionError verifies the context-cancellation fix: client-side timeouts
// and cancellations must NOT be classified as connection errors — the endpoint is
// innocent and must not be penalised or failed-over on an already-expired context.
func TestIsConnectionError(t *testing.T) {
	t.Parallel()

	// A genuine net.Error that is NOT a context error.
	type urlError struct{ msg string }
	_ = urlError{}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context.DeadlineExceeded",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "context.Canceled",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "url.Error wrapping context.DeadlineExceeded",
			err: &net.OpError{
				Op:  "dial",
				Err: context.DeadlineExceeded,
			},
			want: false,
		},
		{
			name: "genuine net.Error (connection reset)",
			err:  &connectionResetError{},
			want: true,
		},
		{
			name: "generic non-network error",
			err:  errors.New("something went wrong"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsConnectionError(tt.err)
			assert.Equal(t, tt.want, got, "IsConnectionError(%v)", tt.err)
		})
	}
}

// TestIsIdempotent confirms the idempotency predicate matches RFC 9110.
func TestIsIdempotent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		want   bool
	}{
		{http.MethodGet, true},
		{http.MethodHead, true},
		{http.MethodOptions, true},
		{http.MethodPost, false},
		{http.MethodPatch, false},
		{http.MethodDelete, false},
		{http.MethodPut, false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isIdempotent(tt.method))
		})
	}
}

// TestResponseStartedWriter_TracksWrites verifies that the sentinel wrapper sets
// started=true on both WriteHeader and Write calls.
func TestResponseStartedWriter_TracksWrites(t *testing.T) {
	t.Parallel()

	t.Run("WriteHeader sets started", func(t *testing.T) {
		t.Parallel()
		rw := &ResponseStartedWriter{ResponseWriter: httptest.NewRecorder()}
		assert.False(t, rw.started)
		rw.WriteHeader(http.StatusOK)
		assert.True(t, rw.started)
	})

	t.Run("Write sets started", func(t *testing.T) {
		t.Parallel()
		rw := &ResponseStartedWriter{ResponseWriter: httptest.NewRecorder()}
		assert.False(t, rw.started)
		_, _ = rw.Write([]byte("hello"))
		assert.True(t, rw.started)
	})

	t.Run("neither called: not started", func(t *testing.T) {
		t.Parallel()
		rw := &ResponseStartedWriter{ResponseWriter: httptest.NewRecorder()}
		assert.False(t, rw.started)
	})
}

// TestRetry_POSTWithBytesWritten_NoRetry is the critical correctness test.
// An httptest backend writes 100 bytes then RSTs. ExecuteWithRetry must NOT
// retry to a second endpoint because the response has already started.
func TestRetry_POSTWithBytesWritten_NoRetry(t *testing.T) {
	t.Parallel()

	attemptsHit := 0

	// Backend: write 100 bytes then close the connection abruptly.
	// We hijack the connection to send a TCP RST after the body begins.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, 100))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Hijack and close to simulate a mid-stream RST.
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	t.Cleanup(srv.Close)

	h := newTestRetryHandler(t)
	ep1 := namedEndpoint("ep1")
	ep2 := namedEndpoint("ep2")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	stats := &ports.RequestStats{}

	proxyFunc := func(ctx context.Context, w http.ResponseWriter, r *http.Request, ep *domain.Endpoint, s *ports.RequestStats) error {
		attemptsHit++
		// Simulate: write the headers + bytes, then return a connection error.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, 100))
		return &connectionResetError{}
	}

	err := h.ExecuteWithRetry(context.Background(), w, req, []*domain.Endpoint{ep1, ep2},
		&roundRobinSelector{}, stats, proxyFunc)

	// Error expected; the stream failed.
	require.Error(t, err)
	// Critically: only ONE attempt. Retrying would double-bill the user.
	assert.Equal(t, 1, attemptsHit, "must not retry POST after response bytes flushed to client")
}

// TestRetry_POSTBeforeBytesWritten_DoesRetry confirms that a connection error
// before any bytes are written still triggers failover, even for POST.
func TestRetry_POSTBeforeBytesWritten_DoesRetry(t *testing.T) {
	t.Parallel()

	attemptsHit := 0

	h := newTestRetryHandler(t)
	ep1 := namedEndpoint("ep1")
	ep2 := namedEndpoint("ep2")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat", nil)
	w := httptest.NewRecorder()
	stats := &ports.RequestStats{}

	proxyFunc := func(ctx context.Context, w http.ResponseWriter, r *http.Request, ep *domain.Endpoint, s *ports.RequestStats) error {
		attemptsHit++
		if attemptsHit == 1 {
			// First attempt: connection error before writing anything.
			return &connectionResetError{}
		}
		// Second attempt: success.
		w.WriteHeader(http.StatusOK)
		return nil
	}

	err := h.ExecuteWithRetry(context.Background(), w, req, []*domain.Endpoint{ep1, ep2},
		&roundRobinSelector{}, stats, proxyFunc)

	require.NoError(t, err)
	assert.Equal(t, 2, attemptsHit, "POST should retry when no bytes have been written yet")
}

// TestRetry_GETMidStreamRST_NoRetry pins the replay fix: once bytes have reached
// the client a GET must not be replayed either, or the client receives the first
// backend's partial output followed by the second backend's full response.
func TestRetry_GETMidStreamRST_NoRetry(t *testing.T) {
	t.Parallel()

	attemptsHit := 0

	h := newTestRetryHandler(t)
	ep1 := namedEndpoint("ep1")
	ep2 := namedEndpoint("ep2")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	stats := &ports.RequestStats{}

	proxyFunc := func(ctx context.Context, w http.ResponseWriter, r *http.Request, ep *domain.Endpoint, s *ports.RequestStats) error {
		attemptsHit++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		return &connectionResetError{}
	}

	err := h.ExecuteWithRetry(context.Background(), w, req, []*domain.Endpoint{ep1, ep2},
		&roundRobinSelector{}, stats, proxyFunc)

	require.Error(t, err)
	assert.Equal(t, 1, attemptsHit, "GET must not be replayed once response bytes were written")
	assert.Equal(t, "partial", w.Body.String(), "client must see only the first attempt's bytes")
}

