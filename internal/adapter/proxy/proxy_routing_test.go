package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// TestProxyRoutingRegression ensures that proxy path stripping works correctly
// This is a regression test for the issue where paths were being double-stripped
// or not stripped at all, causing 404 errors
func TestProxyRoutingRegression(t *testing.T) {
	tests := []struct {
		name                string
		proxyType           string
		requestPath         string
		contextPrefix       string
		expectedBackendPath string
		description         string
	}{
		{
			name:                "Provider proxy with context prefix",
			proxyType:           DefaultProxySherpa,
			requestPath:         "/api/chat",
			contextPrefix:       "/olla/ollama",
			expectedBackendPath: "/api/chat",
			description:         "Provider proxy should use pre-stripped path directly",
		},
		{
			name:                "Regular proxy with context prefix",
			proxyType:           DefaultProxySherpa,
			requestPath:         "/v1/chat/completions",
			contextPrefix:       "/olla/proxy",
			expectedBackendPath: "/v1/chat/completions",
			description:         "Regular proxy should use pre-stripped path directly",
		},
		{
			name:                "Provider proxy with OpenAI path",
			proxyType:           DefaultProxyOlla,
			requestPath:         "/v1/models",
			contextPrefix:       "/olla/openai",
			expectedBackendPath: "/v1/models",
			description:         "OpenAI provider proxy should preserve path correctly",
		},
		{
			name:                "Provider proxy with LM Studio path",
			proxyType:           DefaultProxyOlla,
			requestPath:         "/api/v1/chat/completions",
			contextPrefix:       "/olla/lmstudio",
			expectedBackendPath: "/api/v1/chat/completions",
			description:         "LM Studio provider proxy should preserve path correctly",
		},
		{
			name:                "Ollama provider proxy with api/generate",
			proxyType:           DefaultProxySherpa,
			requestPath:         "/api/generate",
			contextPrefix:       "/olla/ollama",
			expectedBackendPath: "/api/generate",
			description:         "Ollama provider proxy should strip prefix correctly",
		},
		{
			name:                "Ollama provider proxy with api/chat",
			proxyType:           DefaultProxyOlla,
			requestPath:         "/api/chat",
			contextPrefix:       "/olla/ollama",
			expectedBackendPath: "/api/chat",
			description:         "Ollama chat endpoint should work correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test backend server that captures the request path
			var capturedPath string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer backend.Close()

			// Create test endpoint
			testURL, _ := url.Parse(backend.URL)
			endpoint := &domain.Endpoint{
				Name:      "test-endpoint",
				URLString: backend.URL,
				URL:       testURL,
				Type:      "test",
				Status:    domain.StatusHealthy,
			}

			// Setup test components using existing mock infrastructure
			testLogger := createTestLogger()
			mockStats := createTestStatsCollector()
			mockSelector := newMockEndpointSelector(mockStats)
			mockDiscovery := &mockDiscoveryService{
				endpoints: []*domain.Endpoint{endpoint},
			}

			// Create proxy configuration
			config := &Configuration{
				ProxyPrefix:         "/olla",
				ConnectionTimeout:   10 * time.Second,
