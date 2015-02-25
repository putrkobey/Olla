package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/app/middleware"
)

// TestStatusETag_GzipRequestReturnsBare304 is the critical B1xB2 interaction
// assertion: a gzip-negotiated request that hits the ETag path (its second
// sequential poll carries a matching If-None-Match) must return a BARE 304:
//
//   - status 304 Not Modified
//   - ETag header echoing the validator the client sent
//   - NO Content-Encoding header (a 304 has no body to compress)
//   - empty body
//
// This property is what keeps the dashboard's polling loop cheap: the poll
// round-trips a few hundred bytes for a 304 instead of the full gzipped
// payload, and no intermediary sees a Content-Encoding it would try to decode
// against a missing body. All three status surfaces are covered because they
// share the same writeJSONWithETag + middleware.Gzip composition.
func TestStatusETag_GzipRequestReturnsBare304(t *testing.T) {
	t.Parallel()

	app := seedWireShapeApp(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/status", middleware.GzipFunc(app.statusHandler))

	// First poll: negotiate gzip, capture the ETag.
	first := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	first.Header.Set("Accept-Encoding", "gzip")
	firstRec := httptest.NewRecorder()
	mux.ServeHTTP(firstRec, first)
	require.Equal(t, http.StatusOK, firstRec.Code)
	etag := firstRec.Header().Get("ETag")
	require.NotEmpty(t, etag, "first response must emit an ETag")
	require.Equal(t, "gzip", firstRec.Header().Get("Content-Encoding"),
		"first response must be gzipped when the client negotiates it")

	// Second poll: same ETag, still negotiating gzip. The server-side
	// If-None-Match match short-circuits to 304 BEFORE any body is written,
	// so the gzip writer is never engaged and no Content-Encoding leaks onto
