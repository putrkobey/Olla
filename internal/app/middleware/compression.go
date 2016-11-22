package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// gzipMinSize is the body size below which compression is skipped: the gzip
// framing overhead would exceed the savings on tiny payloads, and the three
// handlers this wraps routinely produce multi-KB JSON, so small bodies only
// appear on degenerate fleets where the wire cost is irrelevant anyway.
const gzipMinSize = 256

// maxAcceptEncodingBytes bounds the Accept-Encoding header we are willing to
// parse. A legitimate client sends a short, bounded list of codings; a
// multi-KB header is always pathological (or hostile). Short-circuiting it
// stops an oversized Accept-Encoding from driving an unbounded split before
// the server-level MaxHeaderBytes cap was in place. 1 KiB is far above any
// real browser/curl request, which sits well under 100 bytes.
const maxAcceptEncodingBytes = 1 << 10 // 1 KiB

// gzipLevel is held at DefaultCompression (level 6): a good throughput/ratio
// midpoint for JSON status payloads. BestSpeed saves CPU but inflates large
// endpoint lists; BestCompression is CPU-heavy for a hot polling path.
const gzipLevel = gzip.DefaultCompression

// gzipWriterPool recycles gzip writers across requests. Reset rebinds the
// writer to a fresh ResponseWriter, so a pooled writer is indistinguishable
// from a freshly-allocated one.
var gzipWriterPool = sync.Pool{
	New: func() any {
		return newGzipWriter()
	},
}

func acquireGzipWriter(w io.Writer) *gzip.Writer {
	v, ok := gzipWriterPool.Get().(*gzip.Writer)
	if !ok || v == nil {
		// Pool New never returns anything other than *gzip.Writer, but a
		// checked assertion keeps forcetypeassert happy and is defensive
		// against a future refactor of the pool's New func.
		v = newGzipWriter()
	}
	v.Reset(w)
	return v
}

// newGzipWriter builds a fresh gzip writer at the configured level. Split out
// so acquireGzipWriter can fall back to it on the (unreachable) nil-pool path.
func newGzipWriter() *gzip.Writer {
	w, err := gzip.NewWriterLevel(io.Discard, gzipLevel)
	if err != nil {
		// Unreachable: gzipLevel is a constant valid level. Fall back to the
		// no-arg constructor rather than panicking on the off-chance the
		// constant is somehow invalid at runtime.
		return gzip.NewWriter(io.Discard)
	}
	return w
}

func releaseGzipWriter(gz *gzip.Writer) {
	gzipWriterPool.Put(gz)
}

// Gzip wraps an http.Handler in gzip negotiation. When the client advertises
// Accept-Encoding: gzip (with a non-zero q-value), eligible response bodies are
// compressed and Content-Encoding: gzip is set. The middleware deliberately
// skips compression for: 304 responses (no body, must carry no
// Content-Encoding), text/event-stream responses (streaming SSE must reach the
// client byte-for-byte without buffering), and bodies below gzipMinSize bytes
// (overhead exceeds savings). Vary: Accept-Encoding is set on EVERY response
// from a gzip-wrapped route, including identity/non-gzip responses, so a
// shared cache can never serve a gzipped representation to a client that did
// not negotiate one and believe it is the same resource. Without the
// unconditional Vary a cache keyed only on the request URL would store the
// gzipped variant and hand it verbatim to the next identity-negotiating
// client.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Unconditional: see the doc comment above. Set before the
		// gzip-negotiation branch so identity responses carry Vary too.
		w.Header().Add("Vary", "Accept-Encoding")

		if !acceptsGzip(r.Header.Get(constants.HeaderAcceptEncoding)) {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

// GzipFunc is the http.HandlerFunc adaptor for Gzip, for route registries
// (like Olla's RouteRegistry.RegisterWithMethod) that expect HandlerFunc.
// The wrapped Gzip(h) is built ONCE at registration time rather than inside
// the per-request closure, so a fresh handler wrapper is not allocated on
// every request.
func GzipFunc(h http.HandlerFunc) http.HandlerFunc {
	// http.HandlerFunc satisfies http.Handler, so h passes to Gzip directly
	// without an explicit conversion (which unconvert rightly flags).
	wrapped := Gzip(h)
	return func(w http.ResponseWriter, r *http.Request) {
		wrapped.ServeHTTP(w, r)
	}
}

// acceptsGzip parses the Accept-Encoding header the way RFC 7231 requires:
// each token is examined for an explicit q-value, and gzip counts only when
// its q is present and strictly greater than zero. "gzip;q=0" explicitly
// refuses gzip and must not match. The wildcard "*" matches gzip only when
// gzip itself is not listed with q=0.
//
// Headers longer than maxAcceptEncodingBytes short-circuit to false: a real
// client never sends a multi-KB Accept-Encoding, and the parser splits on ","
// per token, so an oversized header would otherwise drive unbounded work.
// Falling through to "no gzip" preserves correctness (the response is still
// served, just uncompressed) and matches the safe direction.
func acceptsGzip(header string) bool {
	if header == "" || len(header) > maxAcceptEncodingBytes {
		return false
	}
	starQ := float64(-1)
	gzipQ := float64(-1)
