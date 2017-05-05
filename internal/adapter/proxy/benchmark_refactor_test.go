package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/pkg/eventbus"
	"github.com/putrkobey/Olla/pkg/pool"
)

// BenchmarkRefactoredImplementations benchmarks the refactored proxy implementations
func BenchmarkRefactoredImplementations(b *testing.B) {
	b.Run("Sherpa_Refactored", func(b *testing.B) {
		runRefactoredProxyBenchmarks(b, "sherpa")
	})

	b.Run("Olla_Refactored", func(b *testing.B) {
		runRefactoredProxyBenchmarks(b, "olla")
	})
}

func runRefactoredProxyBenchmarks(b *testing.B, proxyType string) {
	b.Run("SimpleRequest", func(b *testing.B) {
		benchmarkRefactoredSimpleRequest(b, proxyType)
	})

	b.Run("ErrorHandling", func(b *testing.B) {
		benchmarkRefactoredErrorHandling(b, proxyType)
	})

	b.Run("HeaderProcessing", func(b *testing.B) {
		benchmarkRefactoredHeaderProcessing(b, proxyType)
	})

	b.Run("StreamingResponse", func(b *testing.B) {
		benchmarkRefactoredStreamingResponse(b, proxyType)
	})

	b.Run("ConfigUpdates", func(b *testing.B) {
		benchmarkRefactoredConfigUpdates(b, proxyType)
	})

	b.Run("StatsCollection", func(b *testing.B) {
		benchmarkRefactoredStatsCollection(b, proxyType)
	})
}

func createRefactoredProxy(proxyType string, endpoints []*domain.Endpoint) (ports.ProxyService, error) {
	discovery := &mockDiscoveryService{endpoints: endpoints}
	selector := &mockEndpointSelector{}
	collector := createTestStatsCollector()
	logger := createTestLogger()

	switch proxyType {
	case "sherpa":
		config := &sherpa.Configuration{}
		config.ResponseTimeout = 30 * time.Second
		config.ReadTimeout = 10 * time.Second
		config.StreamBufferSize = 8192
		return sherpa.NewService(discovery, selector, config, collector, nil, logger)
	case "olla":
		config := &olla.Configuration{}
		config.ResponseTimeout = 30 * time.Second
		config.ReadTimeout = 10 * time.Second
		config.StreamBufferSize = 8192
		config.MaxIdleConns = 200
		config.IdleConnTimeout = 90 * time.Second
		config.MaxConnsPerHost = 50
		return olla.NewService(discovery, selector, config, collector, nil, logger)
	default:
		return nil, fmt.Errorf("unknown proxy type: %s", proxyType)
	}
}

func benchmarkRefactoredSimpleRequest(b *testing.B, proxyType string) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
	proxy, err := createRefactoredProxy(proxyType, []*domain.Endpoint{endpoint})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		stats := &ports.RequestStats{
			RequestID: fmt.Sprintf("bench-%d", i),
			StartTime: time.Now(),
		}

		err := proxy.ProxyRequest(context.Background(), w, req, stats, createTestLogger())
		if err != nil {
			b.Fatal(err)
		}

		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status code: %d", w.Code)
		}
	}
}

func benchmarkRefactoredErrorHandling(b *testing.B, proxyType string) {
	// Failing upstream
	endpoint := createTestEndpoint("test", "http://localhost:99999", domain.StatusHealthy)
	proxy, err := createRefactoredProxy(proxyType, []*domain.Endpoint{endpoint})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		stats := &ports.RequestStats{
			RequestID: fmt.Sprintf("bench-%d", i),
			StartTime: time.Now(),
		}

		_ = proxy.ProxyRequest(context.Background(), w, req, stats, createTestLogger())
	}
}

