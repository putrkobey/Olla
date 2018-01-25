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
	b.ReportAllocs()

	for range b.N {
		req, stats, rlog := createTestRequestWithBody("POST", "/api/query", requestBody)
		w := httptest.NewRecorder()

		err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
	}
}

// benchmarkLargePayload tests performance with large payloads
func benchmarkLargePayload(b *testing.B, suite ProxyTestSuite) {
	// 100KB response
	largeData := strings.Repeat("data", 25000)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeData))
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
		req, stats, rlog := createTestRequestWithBody("GET", "/api/large", "")
		w := httptest.NewRecorder()

		err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
	}
}

// benchmarkStreamingResponse tests streaming performance
func benchmarkStreamingResponse(b *testing.B, suite ProxyTestSuite) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeText)
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)
		for i := range 10 {
			fmt.Fprintf(w, "chunk %d\n", i)
			flusher.Flush()
		}
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
		req, stats, rlog := createTestRequestWithBody("GET", "/api/stream", "")
		w := httptest.NewRecorder()

		err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
	}
}

// benchmarkConcurrentRequests tests performance under concurrent load
func benchmarkConcurrentRequests(b *testing.B, suite ProxyTestSuite) {
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

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, stats, rlog := createTestRequestWithBody("GET", "/api/test", "")
			w := httptest.NewRecorder()

			err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
			if err != nil {
				b.Fatalf("Request failed: %v", err)
			}
		}
	})
}

// benchmarkHighThroughput tests maximum throughput capabilities
func benchmarkHighThroughput(b *testing.B, suite ProxyTestSuite) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Minimal processing time
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

	// Warm up
	for range 100 {
		req, stats, rlog := createTestRequestWithBody("GET", "/api/test", "")
		w := httptest.NewRecorder()
		proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Test with high concurrency
	const workers = 50
	var wg sync.WaitGroup
	requests := make(chan struct{}, b.N)

	for range b.N {
		requests <- struct{}{}
	}
	close(requests)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requests {
				req, stats, rlog := createTestRequestWithBody("GET", "/api/test", "")
				w := httptest.NewRecorder()

				err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
				if err != nil {
					b.Errorf("Request failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

// benchmarkErrorHandling tests error path performance
func benchmarkErrorHandling(b *testing.B, suite ProxyTestSuite) {
	// Use unreachable endpoint to trigger errors
	endpoint := createTestEndpoint("bench", "http://localhost:99999", domain.StatusHealthy)
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

		// This should fail quickly
		err := proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
		if err == nil {
			b.Fatal("Expected error but got none")
		}
	}
}

// benchmarkConfigUpdates tests configuration update performance
func benchmarkConfigUpdates(b *testing.B, suite ProxyTestSuite) {
	discovery := &mockDiscoveryService{}
	selector := &mockEndpointSelector{}
	collector := createTestStatsCollector()
	config := suite.CreateConfig()

	proxy := suite.CreateProxy(discovery, selector, config, collector)

	// Create alternate configs for updates
	var configs []ports.ProxyConfiguration
	if suite.Name() == "Sherpa" {
		for i := range 10 {
			configs = append(configs, &Configuration{
				ResponseTimeout:  time.Duration(i+1) * time.Second,
				ReadTimeout:      time.Duration(i+1) * time.Second,
				StreamBufferSize: (i + 1) * 1024,
			})
		}
	} else {
		for i := range 10 {
			configs = append(configs, &Configuration{
				ResponseTimeout:  time.Duration(i+1) * time.Second,
				ReadTimeout:      time.Duration(i+1) * time.Second,
				StreamBufferSize: (i + 1) * 1024,
				MaxIdleConns:     (i + 1) * 10,
				IdleConnTimeout:  time.Duration(i+1) * 10 * time.Second,
				MaxConnsPerHost:  (i + 1) * 5,
			})
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		configIndex := i % len(configs)
		proxy.UpdateConfig(configs[configIndex])
	}
}

// benchmarkStatsCollection tests statistics collection performance
func benchmarkStatsCollection(b *testing.B, suite ProxyTestSuite) {
	discovery := &mockDiscoveryService{}
	selector := &mockEndpointSelector{}
	collector := createTestStatsCollector()
	config := suite.CreateConfig()

	proxy := suite.CreateProxy(discovery, selector, config, collector)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, err := proxy.GetStats(context.Background())
		if err != nil {
			b.Fatalf("GetStats failed: %v", err)
		}
	}
}
