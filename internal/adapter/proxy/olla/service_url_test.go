package olla

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestBuildTargetURL tests URL building for various scenarios
func TestBuildTargetURL(t *testing.T) {
	config := &Configuration{}
	config.ProxyPrefix = "/olla/"
	config.ResponseTimeout = 30 * time.Second
	config.ReadTimeout = 10 * time.Second
	config.StreamBufferSize = 8192
	config.MaxIdleConns = 200
	config.IdleConnTimeout = 90 * time.Second
	config.MaxConnsPerHost = 50

	service, err := NewService(&mockDiscoveryService{}, &mockEndpointSelector{}, config, &mockStatsCollector{}, nil, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	tests := []struct {
		name          string
		endpointURL   string
		requestPath   string
		requestQuery  string
		expectedPath  string
		expectedQuery string
		expectedHost  string
		useFastPath   bool
		description   string
	}{
		{
			name:          "SimpleEndpoint_NoPath",
			endpointURL:   "http://localhost:11434",
			requestPath:   "/olla/v1/chat/completions",
			requestQuery:  "",
			expectedPath:  "/v1/chat/completions",
			expectedQuery: "",
			expectedHost:  "localhost:11434",
			useFastPath:   true,
			description:   "Common case: Ollama endpoint with no path",
		},
		{
			name:          "SimpleEndpoint_RootPath",
			endpointURL:   "http://localhost:1234/",
			requestPath:   "/olla/v1/completions",
			requestQuery:  "",
			expectedPath:  "/v1/completions",
			expectedQuery: "",
			expectedHost:  "localhost:1234",
			useFastPath:   true,
			description:   "Common case: LM Studio endpoint with / path",
		},
		{
			name:          "SimpleEndpoint_WithQueryString",
			endpointURL:   "http://localhost:11434",
			requestPath:   "/olla/api/tags",
			requestQuery:  "format=json&details=true",
			expectedPath:  "/api/tags",
			expectedQuery: "format=json&details=true",
			expectedHost:  "localhost:11434",
			useFastPath:   true,
			description:   "Query string handling in fast path",
		},
		{
			name:          "ComplexEndpoint_WithPath",
			endpointURL:   "http://localhost:8000/api/v1",
			requestPath:   "/olla/models",
			requestQuery:  "",
			expectedPath:  "/models", // ResolveReference behaviour
			expectedQuery: "",
			expectedHost:  "localhost:8000",
			useFastPath:   false,
			description:   "Rare case: vLLM endpoint with API path",
		},
		{
			name:          "HTTPSEndpoint",
			endpointURL:   "https://api.example.com:443",
			requestPath:   "/olla/v1/embeddings",
			requestQuery:  "model=text-embedding-3-small",
			expectedPath:  "/v1/embeddings",
			expectedQuery: "model=text-embedding-3-small",
			expectedHost:  "api.example.com:443",
			useFastPath:   true,
			description:   "HTTPS endpoint with query string",
		},
		{
			name:          "EmptyProxyPath",
			endpointURL:   "http://localhost:11434",
			requestPath:   "/olla/",
			requestQuery:  "",
			expectedPath:  "/",
			expectedQuery: "",
			expectedHost:  "localhost:11434",
			useFastPath:   true,
			description:   "Edge case: empty path after prefix strip",
		},
	}
