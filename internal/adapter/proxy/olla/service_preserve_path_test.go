package olla

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestService_buildTargetURL_PreservePath tests the basic preserve_path functionality
func TestService_buildTargetURL_PreservePath(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     *domain.Endpoint
		requestPath  string
		proxyPrefix  string
		expectedPath string
		description  string
	}{
		// Backward compatibility tests (preserve_path = false)
		{
			name: "backward_compatibility_no_path",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "localhost:8080",
				},
				PreservePath: false,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/chat/completions",
			description:  "Current behaviour with no endpoint path",
		},
		{
			name: "backward_compatibility_with_slash",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "localhost:8080",
					Path:   "/",
				},
				PreservePath: false,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/chat/completions",
			description:  "Current behaviour with root path",
		},
		{
			name: "backward_compatibility_with_api_path",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "localhost:8080",
					Path:   "/api/v1/",
				},
				PreservePath: false,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/chat/completions",
			description:  "ResolveReference drops base path when preserve_path=false",
		},

		// preserve_path = true tests
		{
			name: "preserve_path_true_with_endpoint_path",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "/v1/api",
				},
				PreservePath: true,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/v1/api/chat/completions",
			description:  "Concatenates paths when preserve_path=true",
		},
		{
			name: "preserve_path_true_with_trailing_slash",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "/api/v1/",
				},
				PreservePath: true,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/api/v1/chat/completions",
			description:  "Handles trailing slashes correctly",
		},
		{
			name: "preserve_path_true_llamacpp_engine",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "localhost:8080",
					Path:   "/engines/llama.cpp/",
				},
				PreservePath: true,
			},
			requestPath:  "/olla/proxy/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/engines/llama.cpp/completions",
			description:  "Real-world llama.cpp engine path",
		},
		{
			name: "preserve_path_false_with_endpoint_path",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "/v1/api",
				},
				PreservePath: false,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/chat/completions",
			description:  "ResolveReference behaviour when preserve_path=false",
		},
		{
			name: "preserve_path_true_but_no_endpoint_path",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "",
				},
				PreservePath: true,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/chat/completions",
			description:  "No path to preserve",
		},
		{
			name: "preserve_path_true_with_root_endpoint_path",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "/",
				},
				PreservePath: true,
			},
			requestPath:  "/olla/proxy/chat/completions",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/chat/completions",
			description:  "Root path special case",
		},
		{
			name: "preserve_path_true_with_nested_paths",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "/api/v2/llm",
				},
				PreservePath: true,
			},
			requestPath:  "/olla/proxy/models/gpt-4/generate",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/api/v2/llm/models/gpt-4/generate",
			description:  "Deep nested paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a minimal service with configuration
			s := &Service{}
			s.configuration.Store(&Configuration{
				OllaConfig: config.OllaConfig{
					BaseProxyConfig: config.BaseProxyConfig{
						ProxyPrefix: tt.proxyPrefix,
					},
				},
			})

			// Create a test request
			req, err := http.NewRequest("POST", tt.requestPath, nil)
			require.NoError(t, err)

			// Build the target URL
			targetURL := common.BuildTargetURL(req, tt.endpoint, s.configuration.Load().GetProxyPrefix())

			// Assert the path is as expected
			assert.Equal(t, tt.expectedPath, targetURL.Path, tt.description)
			assert.Equal(t, tt.endpoint.URL.Scheme, targetURL.Scheme)
			assert.Equal(t, tt.endpoint.URL.Host, targetURL.Host)
		})
	}
}

// TestService_buildTargetURL_EdgeCases tests edge cases and weird paths
func TestService_buildTargetURL_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     *domain.Endpoint
		requestPath  string
		proxyPrefix  string
		expectedPath string
		description  string
	}{
		// Double slash handling
		{
			name: "double_slashes_in_request",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "/api/v1",
				},
				PreservePath: true,
			},
			requestPath:  "/olla/proxy//double//slashes//",
			proxyPrefix:  "/olla/proxy",
			expectedPath: "/api/v1/double/slashes",
			description:  "path.Join normalises double slashes",
		},
		{
			name: "double_slashes_preserve_false",
			endpoint: &domain.Endpoint{
				URL: &url.URL{
					Scheme: "http",
					Host:   "api.example.com",
					Path:   "/api/v1",
				},
				PreservePath: false,
			},
			requestPath:  "/olla/proxy//double//slashes//",
			proxyPrefix:  "/olla/proxy",
