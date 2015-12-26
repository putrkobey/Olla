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
}

// TestStatusHandler_ETagMismatchReturns200 guards the inverse: a non-matching
// If-None-Match must always fall through to a full 200 response with a body.
func TestStatusHandler_ETagMismatchReturns200(t *testing.T) {
	t.Parallel()

	app := seedWireShapeApp(t)

	req := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	req.Header.Set("If-None-Match", `"stale-validator"`)
	w := httptest.NewRecorder()
	app.statusHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.Bytes())
}

// TestStatusHandler_ETagIsWeakAndRoundTrips confirms the ETag the server emits
// is a weak validator (W/"..." prefix) and that a client echoing it verbatim
// still gets a 304. This is the dashboard polling contract under the new
// weak-validator wire format.
func TestStatusHandler_ETagIsWeakAndRoundTrips(t *testing.T) {
	t.Parallel()

	app := seedWireShapeApp(t)

	req1 := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	w1 := httptest.NewRecorder()
	app.statusHandler(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	etag := w1.Header().Get("ETag")
	require.NotEmpty(t, etag)
	assert.True(t, strings.HasPrefix(etag, `W/"`),
		"status ETag must be a weak validator (W/\"...\"), got: %q", etag)

	// Echo the W/"..." value back exactly as a client would.
	req2 := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	app.statusHandler(w2, req2)
	assert.Equal(t, http.StatusNotModified, w2.Code)

	// And confirm a hypothetical strong-form client validator with the same
	// opaque-tag also matches under RFC 7232 weak comparison.
	strong := strings.TrimPrefix(etag, "W/")
	req3 := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	req3.Header.Set("If-None-Match", strong)
	w3 := httptest.NewRecorder()
	app.statusHandler(w3, req3)
	assert.Equal(t, http.StatusNotModified, w3.Code,
		"strong If-None-Match with same opaque-tag must match weak current ETag")
}

// TestStatusResponse_ETagStableAcrossRelativeTimeChurn is the load-bearing
// property of the bespoke hash: only the stable fields feed it, so the ETag
// stays identical across two builds whose Timestamp and relative time-ago
// renderings (uptime, last_check, next_check) have moved. This is exactly the
// shape of two polls taken a second apart against unchanged endpoint state.
func TestStatusResponse_ETagStableAcrossRelativeTimeChurn(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startTime := now.Add(-time.Minute)

	r1 := StatusResponse{
		Timestamp: now,
		Proxy:     ProxySummary{Engine: "olla", Profile: "ollama", Balancer: "priority"},
		System: SystemSummary{
			StartTime:    startTime,
			Status:       statusHealthy,
			EndpointsUp:  "1/1",
			SuccessRate:  "100.0%",
			AvgLatency:   "12ms",
			TotalTraffic: "1.5 MB",
			UptimeHuman:  "1m", // excluded relative string
			Version:      "0.0.29",
			Commit:       "abc",
		},
		Endpoints: []EndpointResponse{{
			ID:            "abc",
			Name:          "ep1",
			Status:        statusHealthy,
			URL:           "http://localhost:11434",
			SuccessRate:   "100.0%",
			LastCheck:     "5s ago", // excluded
			NextCheck:     "in 25s", // excluded
			HealthCheckAt: &now,
			NextCheckAt:   &startTime,
		}},
	}

	// r2 shares no slice backing array with r1: a shallow copy would alias
	// Endpoints and let the relative-string mutation below leak into r1.
