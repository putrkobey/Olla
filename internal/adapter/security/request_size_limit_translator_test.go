package security

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/putrkobey/Olla/internal/config"
)

// countingBody reports how many bytes the middleware pulled from a chunked body, so the tests
// assert the read is bounded rather than inferring it from memory use.
type countingBody struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

func (c *countingBody) Close() error { return nil }

func chunkedRequest(path string, size int) (*http.Request, *countingBody) {
	body := &countingBody{r: strings.NewReader(strings.Repeat("x", size))}
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.ContentLength = -1
	return req, body
}

const (
	translatorServerCap = 1 << 20  // stands in for max_body_size
	translatorCap       = 16 << 10 // stands in for the translator's max_message_size
)

func newTranslatorChain(t *testing.T, reject RejectFunc, translatorLimit int64, handler http.Handler) http.Handler {
	t.Helper()
	sv := createTestSizeLimitValidator(config.ServerRequestLimits{MaxBodySize: translatorServerCap})
	adapters := &Adapters{SizeValidation: sv}
	return adapters.CreateTranslatorChainMiddleware(reject, translatorLimit)(handler)
}

func TestTranslatorChain_ChunkedBodyCappedAtTranslatorLimit(t *testing.T) {
	t.Parallel()

	reached := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })
	var rejected int
	reject := func(w http.ResponseWriter, status int, _ string) {
		rejected = status
		w.WriteHeader(status)
	}
	handler := newTranslatorChain(t, reject, translatorCap, next)

	req, _ := chunkedRequest("/olla/anthropic/v1/messages", translatorCap+1)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge || rejected != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413 through reject, got %d (reject saw %d)", rr.Code, rejected)
	}
	if reached {
		t.Error("handler must not see a chunked body over the translator cap")
	}

	// A body far larger than the server cap must still stop near the translator cap. The
	// slack covers io.ReadAll's final chunk, not a second buffer's worth.
	req, body := chunkedRequest("/olla/anthropic/v1/messages", translatorServerCap-1)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if got := body.n.Load(); got > translatorCap+4096 {
		t.Errorf("read %d bytes of a chunked body; want it bounded near the translator cap %d", got, translatorCap)
	}
}

func TestTranslatorChain_ChunkedBodyUnderCapPasses(t *testing.T) {
	t.Parallel()

	var got int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = len(b)
		w.WriteHeader(http.StatusOK)
	})
	handler := newTranslatorChain(t, nil, translatorCap, next)

	// Exactly at the cap is allowed, matching the handler's own maxBodySize+1 read.
	req, _ := chunkedRequest("/olla/anthropic/v1/messages", translatorCap)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || got != translatorCap {
