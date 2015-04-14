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
