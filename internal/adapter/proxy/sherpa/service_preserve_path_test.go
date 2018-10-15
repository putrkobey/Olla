package sherpa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/pkg/pool"
)

// TestSherpa_PreservePath_URLBuilding tests the URL building logic in Sherpa's proxyToSingleEndpoint
func TestSherpa_PreservePath_URLBuilding(t *testing.T) {
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
			name: "preserve_path_true_with_nested_paths",
			endpoint: &domain.Endpoint{
