package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// funcName extracts the full symbol name of a function value for comparison.
func funcName(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

// newHealthTestLogger returns a quiet logger for transport tests.
func newHealthTestLogger(t *testing.T) logger.StyledLogger {
	t.Helper()
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, cleanup, _ := logger.New(loggerCfg)
	t.Cleanup(cleanup)
	return logger.NewPlainStyledLogger(log)
}

// extractTransport pulls the *http.Transport from the health checker built by
// NewHTTPHealthCheckerWithDefaults. The HTTPClient interface doesn't expose the
// transport, so we type-assert through the concrete *http.Client field.
func extractTransport(t *testing.T) *http.Transport {
	t.Helper()

	checker := NewHTTPHealthCheckerWithDefaults(newMockRepository(), newHealthTestLogger(t))

	// The client field is an HTTPClient interface; NewHTTPHealthCheckerWithDefaults
	// always passes a *http.Client so the assertion is safe in tests.
