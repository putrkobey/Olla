package router

// Regression tests for Phase 3 security hardening item 3:
// Non-proxy routes must receive request-size validation when a sizeMiddlewareProvider
// is wired in, so that oversized bodies are rejected on status/health/stats endpoints.

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/adapter/security"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// capAdapters is a minimal security adapter that applies a 10-byte body cap on
// non-proxy routes (via CreateSizeMiddleware) and a pass-through chain on proxy
// routes (to keep the test focused on item 3 only).
type capAdapters struct{}

func (c *capAdapters) CreateChainMiddleware(security.RejectFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func (c *capAdapters) CreateRateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func (c *capAdapters) CreateSizeMiddleware() func(http.Handler) http.Handler {
	const maxBytes = 10
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TestWireUpWithSecurityChain_NonProxyRouteGetsSizeValidation verifies that a
// non-proxy route rejects an oversized body when a sizeMiddlewareProvider is wired
// in. Without the fix, the route would receive no size check and return 200.
func TestWireUpWithSecurityChain_NonProxyRouteGetsSizeValidation(t *testing.T) {
	t.Parallel()

	reached := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	reg := NewRouteRegistry(&mockRouteLogger{})
	reg.RegisterWithMethod("/internal/status", handler, "status", http.MethodGet)

