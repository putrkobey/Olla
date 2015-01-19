package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
)

// stubHandler returns an HTTP handler that records whether it was invoked and
// always responds 200 OK. Used across CORS wiring tests as the wrapped target.
func stubHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func enabledCORSConfig() config.CorsConfig {
	return config.CorsConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}
}

// TestApplyCORSPreflight verifies that an OPTIONS preflight from an allowed origin
// is answered with CORS headers and does not reach the wrapped handler.
func TestApplyCORSPreflight(t *testing.T) {
	t.Parallel()

	var called bool
	handler := applyCORS(stubHandler(&called), enabledCORSConfig())

	req := httptest.NewRequest(http.MethodOptions, "/olla/proxy/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if called {
		t.Error("wrapped handler must not be reached for preflight OPTIONS")
	}
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", rr.Code)
	}
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v == "" {
		t.Error("expected Access-Control-Allow-Origin on preflight response")
	}
	if v := rr.Header().Get("Access-Control-Allow-Methods"); v == "" {
		t.Error("expected Access-Control-Allow-Methods on preflight response")
	}
}

// TestApplyCORSActualRequest verifies that a real GET from an allowed origin
// reaches the wrapped handler and carries origin + exposed X-Olla-* headers.
func TestApplyCORSActualRequest(t *testing.T) {
	t.Parallel()

	var called bool
	handler := applyCORS(stubHandler(&called), enabledCORSConfig())

	req := httptest.NewRequest(http.MethodGet, "/olla/proxy/v1/models", nil)
	req.Header.Set("Origin", "https://example.com")

