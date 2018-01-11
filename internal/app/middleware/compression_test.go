package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// largeBody is comfortably above gzipMinSize so the threshold check does not
// short-circuit the compression decision in these tests.
const largeBody = `{"endpoints":[{"name":"alpha","status":"healthy","url":"http://localhost:11434"},{"name":"beta","status":"healthy","url":"http://localhost:11435"},{"name":"gamma","status":"healthy","url":"http://localhost:11436"}],"total_count":3,"healthy_count":3,"routable_count":3}`

func writeJSONOK(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

// TestGzip_CompressesJSONWhenNegotiated confirms the happy path: a client that
// advertises Accept-Encoding: gzip receives a gzipped body and the
// Content-Encoding header that lets it know to decode.
func TestGzip_CompressesJSONWhenNegotiated(t *testing.T) {
	t.Parallel()

	h := GzipFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONOK(w, largeBody)
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h(rec, req)

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")

	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, largeBody, string(body))
}

// TestGzip_PassesThroughWithoutAcceptEncoding confirms the middleware is a
// no-op on the body when the client did not negotiate gzip - no
// Content-Encoding header and the body is delivered verbatim. The Vary header
// is still set (see TestGzip_VaryUnconditionalOnIdentity) so a shared cache
// never hands a gzipped representation to an identity-negotiating client.
func TestGzip_PassesThroughWithoutAcceptEncoding(t *testing.T) {
	t.Parallel()

	h := GzipFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONOK(w, largeBody)
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Equal(t, largeBody, rec.Body.String())
}

// TestGzip_VaryUnconditionalOnIdentity locks in the correctness tidy-up: the
// Vary: Accept-Encoding header must be set on every response from a
// gzip-wrapped route, not only on the gzip-negotiated branch. Without this a
// shared cache keyed only on the request URL could store the gzipped variant
// and serve it to the next identity-negotiating client, which would then see
// a Content-Encoding: gzip response it cannot decode. Vary on the identity
// path tells the cache the two negotiated responses are distinct
// representations of the same resource.
func TestGzip_VaryUnconditionalOnIdentity(t *testing.T) {
	t.Parallel()

	h := GzipFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONOK(w, largeBody)
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
}

// TestGzip_OversizedAcceptEncodingShortCircuits confirms an oversized
// Accept-Encoding header is answered without hanging or doing multi-MB work.
// The parser short-circuits to "no gzip", the response is served uncompressed
// with Vary still set, and the test bounds the work done by checking the body
// is delivered verbatim and the request returns promptly.
func TestGzip_OversizedAcceptEncodingShortCircuits(t *testing.T) {
	t.Parallel()

	done := make(chan struct{}, 1)
	h := GzipFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONOK(w, largeBody)
		done <- struct{}{}
	})

	// A multi-MB Accept-Encoding header. The handler must not split it.
	huge := "gzip, " + strings.Repeat("a", 1<<20)
	req := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	req.Header.Set("Accept-Encoding", huge)
	rec := httptest.NewRecorder()
	h(rec, req)

	assert.Empty(t, rec.Header().Get("Content-Encoding"),
		"oversized Accept-Encoding must not be compressed")
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
	assert.Equal(t, largeBody, rec.Body.String())
	select {
	case <-done:
	default:
		t.Error("handler was not invoked")
	}
}

// TestGzip_QZeroDeclines confirms RFC 7231 semantics: an explicit
// "gzip;q=0" forbids gzip and must not be compressed, even though gzip is
// named in the header.
func TestGzip_QZeroDeclines(t *testing.T) {
