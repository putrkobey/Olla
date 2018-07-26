package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// TestProxyPathStripping tests the critical path stripping behavior for both proxy implementations
// This is critical for edge deployments where millions of requests rely on correct routing
//
// NOTE: This test documents the ACTUAL behavior, including known issues that exist in production.
// Some behaviors are not ideal but changing them could break existing clients.
func TestProxyPathStripping(t *testing.T) {
	testCases := []struct {
		name          string
		proxyPrefix   string
		requestPath   string
		contextPrefix string // If set, overrides config prefix via context
		expectedPath  string
		description   string
	}{
		// Normal cases
		{
			name:         "standard_prefix_stripping",
			proxyPrefix:  "/olla",
			requestPath:  "/olla/api/chat",
			expectedPath: "/api/chat",
			description:  "Standard prefix removal",
		},
		{
			name:         "provider_prefix_stripping",
			proxyPrefix:  "/olla/proxy",
			requestPath:  "/olla/proxy/v1/chat/completions",
			expectedPath: "/v1/chat/completions",
			description:  "Provider proxy prefix removal",
		},
		{
			name:         "no_prefix_in_path",
			proxyPrefix:  "/olla",
			requestPath:  "/api/chat",
			expectedPath: "/api/chat",
			description:  "Path without prefix remains unchanged",
		},

		// Edge cases
		{
			name:         "empty_path_after_strip",
			proxyPrefix:  "/olla",
			requestPath:  "/olla",
			expectedPath: "/",
			description:  "Empty path becomes root",
		},
		{
			name:         "prefix_only_partial_match",
			proxyPrefix:  "/olla",
			requestPath:  "/ollama/api/chat",
			expectedPath: "/ma/api/chat", // This is the ACTUAL behavior (a bug!)
			description:  "Partial prefix match incorrectly stripped (BUG)",
		},
		{
			name:         "double_slash_handling",
			proxyPrefix:  "/olla",
			requestPath:  "/olla//api/chat",
			expectedPath: "//api/chat",
			description:  "Double slashes preserved after prefix",
		},
		{
			name:         "trailing_slash_prefix",
			proxyPrefix:  "/olla/",
			requestPath:  "/olla/api/chat",
			expectedPath: "/api/chat",
			description:  "Trailing slash in prefix handled correctly",
		},

		// Security cases - document actual behavior
		{
			name:         "path_traversal_attempt",
			proxyPrefix:  "/olla",
			requestPath:  "/olla/../admin",
			expectedPath: "/admin", // URL.ResolveReference normalizes the path
			description:  "Path traversal normalized by URL resolution (security feature)",
		},
		{
			name:         "encoded_slash_in_path",
			proxyPrefix:  "/olla",
			requestPath:  "/olla/api%2Fchat",
			expectedPath: "/api/chat", // URL decoding happens before stripping
			description:  "URL encoded characters decoded by HTTP library",
		},
	}

	// Test both proxy implementations
	proxyTypes := []struct {
		name    string
		factory func(config ports.ProxyConfiguration) (ports.ProxyService, error)
	}{
		{
			name: "Sherpa",
			factory: func(config ports.ProxyConfiguration) (ports.ProxyService, error) {
				discovery := &mockDiscoveryService{}
				selector := &mockEndpointSelector{}
				return sherpa.NewService(discovery, selector, config.(*sherpa.Configuration), createTestStatsCollector(), nil, createTestLogger())
			},
		},
		{
			name: "Olla",
			factory: func(config ports.ProxyConfiguration) (ports.ProxyService, error) {
				discovery := &mockDiscoveryService{}
				selector := &mockEndpointSelector{}
				return olla.NewService(discovery, selector, config.(*olla.Configuration), createTestStatsCollector(), nil, createTestLogger())
			},
		},
	}

	for _, proxyType := range proxyTypes {
		t.Run(proxyType.name, func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// Create upstream server that echoes the received path
					upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("X-Received-Path", r.URL.Path)
						w.WriteHeader(http.StatusOK)
						fmt.Fprint(w, r.URL.Path)
					}))
					defer upstream.Close()

					// Create proxy configuration
					var config ports.ProxyConfiguration
					if proxyType.name == "Sherpa" {
						sherpaConfig := &sherpa.Configuration{}
						sherpaConfig.ProxyPrefix = tc.proxyPrefix
						config = sherpaConfig
					} else {
						ollaConfig := &olla.Configuration{}
						ollaConfig.ProxyPrefix = tc.proxyPrefix
						config = ollaConfig
					}

					// Create proxy service
					proxy, err := proxyType.factory(config)
					if err != nil {
						t.Fatalf("Failed to create proxy: %v", err)
					}

					// Set up endpoint
					endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
					endpoints := []*domain.Endpoint{endpoint}

					// Create request
					req := httptest.NewRequest("GET", tc.requestPath, nil)

					// Add context override if specified
					if tc.contextPrefix != "" {
						ctx := context.WithValue(req.Context(), tc.proxyPrefix, tc.contextPrefix)
						req = req.WithContext(ctx)
					}

					// Execute request
					w := httptest.NewRecorder()
					stats := &ports.RequestStats{
						RequestID: "test-" + tc.name,
						StartTime: time.Now(),
					}
					logger := createTestLogger()

