package services

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/discovery"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func discoveryTestLogger(t *testing.T) logger.StyledLogger {
	t.Helper()
	log, cleanup, err := logger.New(&logger.Config{Level: "error", Theme: "default"})
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return logger.NewPlainStyledLogger(log)
}

// dialTo sends every connection except the one to proxyAddr to realAddr, so a
// request addressed to a non-loopback host reaches a local test server. Go never
// consults the environment proxy for loopback targets, so the endpoint URL must be
// non-loopback for the proxy decision to be exercised at all.
func dialTo(realAddr, proxyAddr string) func(context.Context, string, string) (net.Conn, error) {
	var d net.Dialer
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == proxyAddr {
			return d.DialContext(ctx, network, addr)
		}
		return d.DialContext(ctx, network, realAddr)
	}
}

func ollamaEndpoint(t *testing.T, rawURL string) *domain.Endpoint {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return &domain.Endpoint{
		Name: "ollama", URL: u, URLString: u.String(), Type: domain.ProfileOllama,
		AuthHeaderName: "X-Api-Key", AuthHeaderValue: "sk-SECRET-123",
		Headers: map[string]string{"X-Tenant": "acme"},
	}
}

// Not parallel: it sets HTTP_PROXY for the process (Go reads it once, so the control uses an explicit proxy).
func TestDiscoveryClient_IgnoresEnvironmentProxy(t *testing.T) {
	var proxied atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxied.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(proxy.Close)

	var direct atomic.Int32
	var gotKey atomic.Value
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		direct.Add(1)
		gotKey.Store(r.Header.Get("X-Api-Key"))
