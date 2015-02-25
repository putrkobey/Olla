package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
)

// nextCalled is a simple flag handler used across CORS tests to detect whether
// the next handler in the chain was reached.
func nextCalledHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func baseCORSConfig() config.CorsConfig {
	return config.CorsConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}
}

// TestCORSPreflight verifies that a preflight OPTIONS request is answered by
// rs/cors directly (204, CORS headers set) without reaching the next handler.
func TestCORSPreflight(t *testing.T) {
	t.Parallel()

	var called bool
	c := NewCORS(baseCORSConfig())
	handler := c.Handler(nextCalledHandler(&called))

	req := httptest.NewRequest(http.MethodOptions, "/olla/proxy/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if called {
		t.Error("next handler must not be called for preflight requests")
	}
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected Access-Control-Allow-Origin to be set on preflight response")
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods to be set on preflight response")
	}
}

// TestCORSActualRequest verifies that a real GET from an allowed origin passes
// through to the next handler and that the response carries both the origin and
// the default exposed X-Olla-* headers.
func TestCORSActualRequest(t *testing.T) {
	t.Parallel()

	var called bool
	c := NewCORS(baseCORSConfig())
	handler := c.Handler(nextCalledHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/olla/proxy/v1/models", nil)
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler must be called for actual (non-preflight) requests")
	}
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected Access-Control-Allow-Origin on actual request response")
	}

	exposed := rr.Header().Get("Access-Control-Expose-Headers")
	for _, h := range []string{constants.HeaderXOllaModel, constants.HeaderXOllaEndpoint} {
		if !strings.Contains(exposed, h) {
			t.Errorf("expected Access-Control-Expose-Headers to contain %q, got %q", h, exposed)
		}
	}
}

// TestCORSNoOriginPassthrough verifies that requests without an Origin header
// are passed through unchanged with no CORS headers added.
func TestCORSNoOriginPassthrough(t *testing.T) {
	t.Parallel()

	var called bool
	c := NewCORS(baseCORSConfig())
	handler := c.Handler(nextCalledHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/internal/health", nil)
	// Deliberately no Origin header — simulates curl, SDK, or server-to-server calls.

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler must be called when no Origin header is present")
	}
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for non-CORS request, got %q", v)
	}
	if v := rr.Header().Get("Access-Control-Expose-Headers"); v != "" {
		t.Errorf("expected no Access-Control-Expose-Headers for non-CORS request, got %q", v)
	}
