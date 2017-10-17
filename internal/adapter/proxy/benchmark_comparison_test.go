package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// BenchmarkProxyComparison compares the refactored implementations
func BenchmarkProxyComparison(b *testing.B) {
	// Set up test server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
	endpoints := []*domain.Endpoint{endpoint}

	// Create refactored proxies
	refactoredSherpa, err := createRefactoredSherpaProxy(endpoints)
	if err != nil {
		b.Fatal(err)
	}

	refactoredOlla, err := createRefactoredOllaProxy(endpoints)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	b.Run("Sherpa_Refactored", func(b *testing.B) {
		benchmarkProxy(b, refactoredSherpa)
	})

	b.Run("Olla_Refactored", func(b *testing.B) {
		benchmarkProxy(b, refactoredOlla)
	})
}

func createRefactoredSherpaProxy(endpoints []*domain.Endpoint) (ports.ProxyService, error) {
	discovery := &mockDiscoveryService{endpoints: endpoints}
	selector := &mockEndpointSelector{}
	collector := createTestStatsCollector()
	logger := createTestLogger()

	config := &sherpa.Configuration{}
	config.ResponseTimeout = 30 * time.Second
	config.ReadTimeout = 10 * time.Second
	config.StreamBufferSize = 8192
	return sherpa.NewService(discovery, selector, config, collector, nil, logger)
}

func createRefactoredOllaProxy(endpoints []*domain.Endpoint) (ports.ProxyService, error) {
	discovery := &mockDiscoveryService{endpoints: endpoints}
	selector := &mockEndpointSelector{}
	collector := createTestStatsCollector()
	logger := createTestLogger()

	config := &olla.Configuration{}
	config.ResponseTimeout = 30 * time.Second
	config.ReadTimeout = 10 * time.Second
	config.StreamBufferSize = 8192
	config.MaxIdleConns = 200
	config.IdleConnTimeout = 90 * time.Second
	config.MaxConnsPerHost = 50
	return olla.NewService(discovery, selector, config, collector, nil, logger)
