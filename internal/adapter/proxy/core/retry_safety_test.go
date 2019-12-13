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
