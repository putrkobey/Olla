package olla

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// BenchmarkBuildTargetURL benchmarks URL building with fast path vs slow path
func BenchmarkBuildTargetURL(b *testing.B) {
	// Create a test service
	config := &Configuration{}
	config.ResponseTimeout = 30 * time.Second
	config.ReadTimeout = 10 * time.Second
	config.StreamBufferSize = 8192
	config.MaxIdleConns = 200
	config.IdleConnTimeout = 90 * time.Second
	config.MaxConnsPerHost = 50

	service, err := NewService(&mockDiscoveryService{}, &mockEndpointSelector{}, config, &mockStatsCollector{}, nil, createTestLogger())
	if err != nil {
		b.Fatalf("failed to create service: %v", err)
	}

	b.Run("FastPath_SimpleEndpoint", func(b *testing.B) {
		// Test fast path: endpoint with no path (majority of use cases)
		endpointURL, _ := url.Parse("http://localhost:11434")
		endpoint := &domain.Endpoint{
			Name: "ollama",
			URL:  endpointURL,
		}

		req, _ := http.NewRequest("POST", "/olla/v1/chat/completions?model=llama3", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for range b.N {
			targetURL := common.BuildTargetURL(req, endpoint, service.configuration.Load().GetProxyPrefix())
			if targetURL.Path != "/v1/chat/completions" {
				b.Fatalf("unexpected path: %s", targetURL.Path)
			}
			if targetURL.RawQuery != "model=llama3" {
				b.Fatalf("unexpected query: %s", targetURL.RawQuery)
			}
		}
	})

	b.Run("FastPath_RootPath", func(b *testing.B) {
		// Test fast path: endpoint with "/" path
		endpointURL, _ := url.Parse("http://localhost:1234/")
		endpoint := &domain.Endpoint{
			Name: "lmstudio",
			URL:  endpointURL,
		}

		req, _ := http.NewRequest("POST", "/olla/v1/completions", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for range b.N {
			targetURL := common.BuildTargetURL(req, endpoint, service.configuration.Load().GetProxyPrefix())
			if targetURL.Path != "/v1/completions" {
				b.Fatalf("unexpected path: %s", targetURL.Path)
			}
		}
