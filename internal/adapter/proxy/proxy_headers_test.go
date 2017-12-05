package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestProxyResponseHeaders(t *testing.T) {
	// Create test upstream server that returns some headers
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.Header().Set("X-Upstream-Header", "upstream-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	// Test both proxy implementations
	testCases := []struct {
		name       string
		createFunc func() ports.ProxyService
	}{
		{
			name: "Sherpa",
			createFunc: func() ports.ProxyService {
				endpoint := createTestEndpoint("test-endpoint", upstream.URL, domain.StatusHealthy)
				discovery := &mockDiscoveryService{endpoints: []*domain.Endpoint{endpoint}}
				selector := &mockEndpointSelector{endpoint: endpoint}
				config := &sherpa.Configuration{}
				proxy, _ := sherpa.NewService(discovery, selector, config, createTestStatsCollector(), nil, createTestLogger())
				return proxy
			},
		},
		{
			name: "Olla",
			createFunc: func() ports.ProxyService {
				endpoint := createTestEndpoint("test-endpoint", upstream.URL, domain.StatusHealthy)
				discovery := &mockDiscoveryService{endpoints: []*domain.Endpoint{endpoint}}
				selector := &mockEndpointSelector{endpoint: endpoint}
				config := &olla.Configuration{}
				proxy, _ := olla.NewService(discovery, selector, config, createTestStatsCollector(), nil, createTestLogger())
				return proxy
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := tc.createFunc()

			// Test without model in context
			t.Run("without model", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()
				stats := &ports.RequestStats{RequestID: "test-req-123"}
				rlog := createTestLogger()

				err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
				assert.NoError(t, err)

				// Check our custom headers
				assert.Equal(t, "test-endpoint", w.Header().Get(constants.HeaderXOllaEndpoint))
				assert.NotEmpty(t, w.Header().Get(constants.HeaderXServedBy)) // Contains version info
				assert.Contains(t, w.Header().Get(constants.HeaderXServedBy), "Olla/")
				assert.Empty(t, w.Header().Get(constants.HeaderXOllaModel))
				assert.NotEmpty(t, w.Header().Get(constants.HeaderXOllaRequestID))
				assert.NotEmpty(t, w.Header().Get(constants.HeaderXOllaBackendType))

				// Check upstream headers are preserved
				assert.Equal(t, constants.ContentTypeJSON, w.Header().Get(constants.HeaderContentType))
				assert.Equal(t, "upstream-value", w.Header().Get("X-Upstream-Header"))
			})

			// Test with model in context
			t.Run("with model", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test", nil)
				ctx := context.WithValue(req.Context(), constants.ContextModelKey, "llama3.2:3b")
				req = req.WithContext(ctx)

				w := httptest.NewRecorder()
				stats := &ports.RequestStats{RequestID: "test-req-456"}
				rlog := createTestLogger()

				err := proxy.ProxyRequest(ctx, w, req, stats, rlog)
				assert.NoError(t, err)

				// Check our custom headers
				assert.Equal(t, "test-endpoint", w.Header().Get(constants.HeaderXOllaEndpoint))
				assert.NotEmpty(t, w.Header().Get(constants.HeaderXServedBy)) // Contains version info
				assert.Contains(t, w.Header().Get(constants.HeaderXServedBy), "Olla/")
				assert.Equal(t, "llama3.2:3b", w.Header().Get(constants.HeaderXOllaModel))
				assert.NotEmpty(t, w.Header().Get(constants.HeaderXOllaRequestID))
				assert.NotEmpty(t, w.Header().Get(constants.HeaderXOllaBackendType))

				// Check upstream headers are preserved
				assert.Equal(t, constants.ContentTypeJSON, w.Header().Get(constants.HeaderContentType))
				assert.Equal(t, "upstream-value", w.Header().Get("X-Upstream-Header"))
			})
		})
	}
}

// Test that our headers can't be overridden by upstream
func TestProxyResponseHeaders_NoOverride(t *testing.T) {
	// Create test upstream that tries to set our headers
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderXOllaEndpoint, "fake-endpoint")
		w.Header().Set(constants.HeaderXOllaModel, "fake-model")
		w.Header().Set(constants.HeaderXServedBy, "fake-server")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("real-endpoint", upstream.URL, domain.StatusHealthy)
	discovery := &mockDiscoveryService{endpoints: []*domain.Endpoint{endpoint}}
	selector := &mockEndpointSelector{endpoint: endpoint}
	config := &sherpa.Configuration{}
	proxy, _ := sherpa.NewService(discovery, selector, config, createTestStatsCollector(), nil, createTestLogger())

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), constants.ContextModelKey, "real-model")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	stats := &ports.RequestStats{RequestID: "test-req-789"}
	rlog := createTestLogger()

	err := proxy.ProxyRequest(ctx, w, req, stats, rlog)
	assert.NoError(t, err)

	// Our headers should NOT be overridden
	assert.Equal(t, "real-endpoint", w.Header().Get(constants.HeaderXOllaEndpoint))
	assert.Equal(t, "real-model", w.Header().Get(constants.HeaderXOllaModel))
	assert.NotEmpty(t, w.Header().Get(constants.HeaderXServedBy))
	assert.Contains(t, w.Header().Get(constants.HeaderXServedBy), "Olla/")
}

// hostHeaderSuites returns both proxy engines paired with a constructor so tests
// can iterate over both without duplicating setup.
func hostHeaderSuites() []struct {
	name        string
	createProxy func(upstream *httptest.Server) ports.ProxyService
} {
	make := func(name string, fn func(upstream *httptest.Server) ports.ProxyService) struct {
		name        string
		createProxy func(upstream *httptest.Server) ports.ProxyService
	} {
		return struct {
			name        string
			createProxy func(upstream *httptest.Server) ports.ProxyService
		}{name, fn}
	}

	return []struct {
		name        string
		createProxy func(upstream *httptest.Server) ports.ProxyService
	}{
		make("Sherpa", func(upstream *httptest.Server) ports.ProxyService {
			ep := createTestEndpoint("be", upstream.URL, domain.StatusHealthy)
			disc := &mockDiscoveryService{endpoints: []*domain.Endpoint{ep}}
			sel := &mockEndpointSelector{endpoint: ep}
			cfg := &sherpa.Configuration{} // zero value is fine for unit tests
			svc, _ := sherpa.NewService(disc, sel, cfg, createTestStatsCollector(), nil, createTestLogger())
			return svc
		}),
		make("Olla", func(upstream *httptest.Server) ports.ProxyService {
			ep := createTestEndpoint("be", upstream.URL, domain.StatusHealthy)
			disc := &mockDiscoveryService{endpoints: []*domain.Endpoint{ep}}
			sel := &mockEndpointSelector{endpoint: ep}
			cfg := &olla.Configuration{}
			svc, _ := olla.NewService(disc, sel, cfg, createTestStatsCollector(), nil, createTestLogger())
			return svc
		}),
	}
}

// TestProxy_OutboundHostMatchesBackend asserts that the Host seen by the backend
// is its own address, not the inbound client Host. Regression for issue #135.
func TestProxy_OutboundHostMatchesBackend(t *testing.T) {
	t.Parallel()

	for _, tc := range hostHeaderSuites() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var capturedHost string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedHost = r.Host
				w.WriteHeader(http.StatusOK)
			}))
			defer upstream.Close()

			proxy := tc.createProxy(upstream)

			req := httptest.NewRequest("GET", "/test", nil)
			req.Host = "olla.example.com" // inbound client host — must NOT reach the backend

			stats := &ports.RequestStats{RequestID: "host-test", StartTime: time.Now()}
			w := httptest.NewRecorder()
			_ = proxy.ProxyRequest(req.Context(), w, req, stats, createTestLogger())

			// The backend's listener address is the authoritative host.
			assert.Equal(t, upstream.Listener.Addr().String(), capturedHost,
				"backend must receive its own address as Host, not the inbound client Host")
		})
	}
}

// TestProxy_XForwardedHostPreservesInbound asserts that backends can still recover the
// original client Host via X-Forwarded-Host even though we don't propagate it as Host.
func TestProxy_XForwardedHostPreservesInbound(t *testing.T) {
	t.Parallel()

