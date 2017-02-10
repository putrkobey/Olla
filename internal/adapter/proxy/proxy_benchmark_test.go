package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/health"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// BenchmarkProxyImplementations benchmarks both proxy implementations
func BenchmarkProxyImplementations(b *testing.B) {
	suites := []ProxyTestSuite{
		SherpaTestSuite{},
		OllaTestSuite{},
	}

	for _, suite := range suites {
		b.Run(suite.Name(), func(b *testing.B) {
			runProxyBenchmarks(b, suite)
		})
	}
}

func runProxyBenchmarks(b *testing.B, suite ProxyTestSuite) {
	b.Run("SimpleRequest", func(b *testing.B) {
		benchmarkSimpleRequest(b, suite)
	})

	b.Run("SmallPayload", func(b *testing.B) {
		benchmarkSmallPayload(b, suite)
	})

	b.Run("LargePayload", func(b *testing.B) {
		benchmarkLargePayload(b, suite)
	})

	b.Run("StreamingResponse", func(b *testing.B) {
		benchmarkStreamingResponse(b, suite)
	})

	b.Run("ConcurrentRequests", func(b *testing.B) {
		benchmarkConcurrentRequests(b, suite)
	})

	b.Run("HighThroughput", func(b *testing.B) {
		benchmarkHighThroughput(b, suite)
	})

	b.Run("ErrorHandling", func(b *testing.B) {
		benchmarkErrorHandling(b, suite)
	})

	b.Run("ConfigUpdates", func(b *testing.B) {
		benchmarkConfigUpdates(b, suite)
	})

	b.Run("StatsCollection", func(b *testing.B) {
		benchmarkStatsCollection(b, suite)
	})

	b.Run("HeaderProcessing", func(b *testing.B) {
		benchmarkHeaderProcessing(b, suite)
	})
}

// benchmarkSimpleRequest tests basic request/response performance
func benchmarkSimpleRequest(b *testing.B, suite ProxyTestSuite) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("bench", upstream.URL, domain.StatusHealthy)
	discovery := &mockDiscoveryService{endpoints: []*domain.Endpoint{endpoint}}
	selector := &mockEndpointSelector{endpoint: endpoint}
	collector := createTestStatsCollector()
	config := suite.CreateConfig()

	proxy := suite.CreateProxy(discovery, selector, config, collector)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		req, stats, rlog := createTestRequestWithBody("GET", "/api/test", "")
		w := httptest.NewRecorder()

		err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
	}
}

// benchmarkSmallPayload tests performance with small JSON payloads
func benchmarkSmallPayload(b *testing.B, suite ProxyTestSuite) {
	responseData := `{"status": "ok", "data": {"id": 123, "name": "test"}}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseData))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("bench", upstream.URL, domain.StatusHealthy)
	discovery := &mockDiscoveryService{endpoints: []*domain.Endpoint{endpoint}}
	selector := &mockEndpointSelector{endpoint: endpoint}
	collector := createTestStatsCollector()
	config := suite.CreateConfig()

	proxy := suite.CreateProxy(discovery, selector, config, collector)

	requestBody := `{"query": "benchmark test", "limit": 10}`

	b.ResetTimer()
