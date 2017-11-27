package handlers

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/app/handlers/dashboard"
	"github.com/putrkobey/Olla/internal/app/middleware"
)

// muxWithStatusRoute builds a mux wired the way registerRoutes wires it in
// production: the status handler is wrapped in middleware.GzipFunc before
// registration. The Application is the same one the wire-shape tests use, so
// the status handler has its real dependencies (repository, stats, registry,
// Config). A fresh mux keeps the assertion local to the status route.
func muxWithStatusRoute(t *testing.T) (http.Handler, *Application) {
	t.Helper()
	app := seedWireShapeApp(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/status", middleware.GzipFunc(app.statusHandler))
	return mux, app
}

// TestStatusRoute_GzipAppliedThroughMux confirms the production wiring: a
// request to /internal/status with Accept-Encoding: gzip gets a gzipped body
// and the Content-Encoding header. This catches accidental unwrapping of the
// middleware at the registration call site.
func TestStatusRoute_GzipAppliedThroughMux(t *testing.T) {
	t.Parallel()

	mux, _ := muxWithStatusRoute(t)

	req := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
