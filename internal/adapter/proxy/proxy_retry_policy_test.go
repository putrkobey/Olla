package proxy

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// resetBackend reads the whole request, then (optionally after sending a partial
// body) closes the connection with a TCP RST, the way a backend that crashes
// mid-request does. hits counts requests that reached it.
func resetBackend(t *testing.T, partial string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	hits := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		if partial != "" {
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(partial))
