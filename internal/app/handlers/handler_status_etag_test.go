package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusHandler_ETag304OnMatch confirms the dashboard's polling contract:
// a first request emits an ETag with a 200, the second request carrying that
// ETag as If-None-Match gets a bare 304 with no body and the same ETag echoed.
func TestStatusHandler_ETag304OnMatch(t *testing.T) {
	t.Parallel()

	app := seedWireShapeApp(t)

	req1 := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	w1 := httptest.NewRecorder()
	app.statusHandler(w1, req1)

	require.Equal(t, http.StatusOK, w1.Code)
	etag := w1.Header().Get("ETag")
	require.NotEmpty(t, etag, "first response must carry an ETag")
	assert.Contains(t, w1.Header().Get("Content-Type"), "application/json")
	assert.Equal(t, "private, no-cache", w1.Header().Get("Cache-Control"), "200 must not be shared-cacheable")

	req2 := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	app.statusHandler(w2, req2)

	assert.Equal(t, http.StatusNotModified, w2.Code)
	assert.Equal(t, etag, w2.Header().Get("ETag"))
	assert.Empty(t, w2.Body.Bytes(), "304 must carry no body")
	assert.Equal(t, "private, no-cache", w2.Header().Get("Cache-Control"), "304 must also carry Cache-Control")
}

// TestEndpointsStatusHandler_ETag304OnMatch is the same contract for the
// /internal/status/endpoints surface.
func TestEndpointsStatusHandler_ETag304OnMatch(t *testing.T) {
	t.Parallel()

	app := seedWireShapeApp(t)

	req1 := httptest.NewRequest(http.MethodGet, "/internal/status/endpoints", nil)
	w1 := httptest.NewRecorder()
	app.endpointsStatusHandler(w1, req1)

	require.Equal(t, http.StatusOK, w1.Code)
	etag := w1.Header().Get("ETag")
	require.NotEmpty(t, etag)
	assert.Equal(t, "private, no-cache", w1.Header().Get("Cache-Control"))

	req2 := httptest.NewRequest(http.MethodGet, "/internal/status/endpoints", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	app.endpointsStatusHandler(w2, req2)

	assert.Equal(t, http.StatusNotModified, w2.Code)
	assert.Equal(t, etag, w2.Header().Get("ETag"))
	assert.Empty(t, w2.Body.Bytes())
	assert.Equal(t, "private, no-cache", w2.Header().Get("Cache-Control"))
}

// TestModelsStatusHandler_ETag304OnMatch is the same contract for the
// /internal/status/models surface.
func TestModelsStatusHandler_ETag304OnMatch(t *testing.T) {
	t.Parallel()

	app := seedWireShapeApp(t)

	req1 := httptest.NewRequest(http.MethodGet, "/internal/status/models", nil)
	w1 := httptest.NewRecorder()
	app.modelsStatusHandler(w1, req1)

	require.Equal(t, http.StatusOK, w1.Code)
	etag := w1.Header().Get("ETag")
	require.NotEmpty(t, etag)
	assert.Equal(t, "private, no-cache", w1.Header().Get("Cache-Control"))

	req2 := httptest.NewRequest(http.MethodGet, "/internal/status/models", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	app.modelsStatusHandler(w2, req2)

	assert.Equal(t, http.StatusNotModified, w2.Code)
	assert.Equal(t, etag, w2.Header().Get("ETag"))
	assert.Empty(t, w2.Body.Bytes())
	assert.Equal(t, "private, no-cache", w2.Header().Get("Cache-Control"))
