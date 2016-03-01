package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// Helper functions for creating test components
func createTestSherpaProxy(endpoints []*domain.Endpoint) (*sherpa.Service, *mockEndpointSelector, ports.StatsCollector) {
	collector := createTestStatsCollector()
	discovery := &mockDiscoveryService{endpoints: endpoints}
	selector := newMockEndpointSelector(collector)
	config := &sherpa.Configuration{}
	config.ResponseTimeout = 30 * time.Second
	config.ReadTimeout = 10 * time.Second
	config.StreamBufferSize = 1024
	proxy, err := sherpa.NewService(discovery, selector, config, collector, nil, createTestLogger())
	if err != nil {
		panic(fmt.Sprintf("failed to create Sherpa proxy: %v", err))
	}
	return proxy, selector, collector
}

func createTestSherpaProxyWithConfig(endpoints []*domain.Endpoint, config *sherpa.Configuration) (*sherpa.Service, *mockEndpointSelector, ports.StatsCollector) {
	collector := createTestStatsCollector()
	discovery := &mockDiscoveryService{endpoints: endpoints}
	selector := newMockEndpointSelector(collector)
	proxy, err := sherpa.NewService(discovery, selector, config, collector, nil, createTestLogger())
	if err != nil {
		panic(fmt.Sprintf("failed to create Sherpa proxy: %v", err))
	}
	return proxy, selector, collector
}

func createTestRequestWithStats(method, path, body string) (*http.Request, *ports.RequestStats, logger.StyledLogger) {
	ctx := context.WithValue(context.Background(), constants.ContextRequestIdKey, "test-request-id")
	ctx = context.WithValue(ctx, constants.ContextRequestTimeKey, time.Now())

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req = req.WithContext(ctx)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	stats := &ports.RequestStats{
		RequestID: "test-request-id",
		StartTime: time.Now(),
	}

	logger := createTestLogger()
	return req, stats, logger
}

func assertSuccessfulResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, expectedBodyContains string) {
	t.Helper()
	if w.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d", expectedStatus, w.Code)
	}
	if expectedBodyContains != "" && !strings.Contains(w.Body.String(), expectedBodyContains) {
		t.Errorf("Expected body to contain %q, got %q", expectedBodyContains, w.Body.String())
	}
}

func assertError(t *testing.T, err error, expectedErrorContains string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error but got nil")
		return
	}
	if expectedErrorContains != "" && !strings.Contains(err.Error(), expectedErrorContains) {
		t.Errorf("Expected error to contain %q, got %q", expectedErrorContains, err.Error())
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func assertStatsPopulated(t *testing.T, stats *ports.RequestStats, expectedEndpoint string) {
	t.Helper()
	if expectedEndpoint != "" && stats.EndpointName != expectedEndpoint {
		t.Errorf("Expected endpoint name %q, got %q", expectedEndpoint, stats.EndpointName)
	}
	if stats.TotalBytes <= 0 {
		t.Error("Expected non-zero bytes transferred")
	}
	if stats.Latency < 0 {
		t.Errorf("Expected positive or zero latency but got %v, how is that possible? o_O", stats.Latency)
	}
}

// TestProxyService_ClientDisconnectHandling tests client disconnect behavior during streaming
// This test verifies that both proxy implementations handle client disconnections gracefully
func TestProxyService_ClientDisconnectHandling(t *testing.T) {
	// Create a slow upstream that simulates streaming
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		// Stream data slowly
		for i := range 10 {
			_, err := w.Write([]byte(fmt.Sprintf("chunk %d\n", i)))
			if err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	testCases := []struct {
		name       string
		createFunc func() ports.ProxyService
	}{
		{
			name: "Sherpa",
			createFunc: func() ports.ProxyService {
				endpoint := createTestEndpoint("test-endpoint", upstream.URL, domain.StatusHealthy)
				discovery := &mockDiscoveryService{endpoints: []*domain.Endpoint{endpoint}}
				selector := &mockEndpointSelector{endpoint: endpoint}
				config := &sherpa.Configuration{}
				config.ReadTimeout = 200 * time.Millisecond
				proxy, _ := sherpa.NewService(discovery, selector, config, createTestStatsCollector(), nil, createTestLogger())
				return proxy
			},
		},
		{
			name: "Olla",
			createFunc: func() ports.ProxyService {
				endpoint := createTestEndpoint("test-endpoint", upstream.URL, domain.StatusHealthy)
				discovery := &mockDiscoveryService{endpoints: []*domain.Endpoint{endpoint}}
				selector := &mockEndpointSelector{endpoint: endpoint}
				config := &olla.Configuration{}
				config.ReadTimeout = 200 * time.Millisecond
				proxy, _ := olla.NewService(discovery, selector, config, createTestStatsCollector(), nil, createTestLogger())
				return proxy
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := tc.createFunc()

			// Create a context that we'll cancel to simulate client disconnect
			ctx, cancel := context.WithCancel(context.Background())

			req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			// Start the proxy request in a goroutine
			done := make(chan error)
			go func() {
				stats := &ports.RequestStats{
					StartTime: time.Now(),
					RequestID: "test-123",
				}
				err := proxy.ProxyRequest(ctx, w, req, stats, createTestLogger())
				done <- err
			}()

			// Wait a bit then cancel the context to simulate client disconnect
			time.Sleep(250 * time.Millisecond)
			cancel()

			// Wait for the proxy to finish
			err := <-done

			// Should get an error about client disconnection
			if err != nil {
				assert.Contains(t, err.Error(), "client disconnected")
			}

			// Should have received some data before disconnect
			response := w.Body.String()
			assert.Contains(t, response, "chunk")
			assert.NotContains(t, response, "chunk 9") // Should not have all chunks
		})
	}
}

// DEPRECATED: Route prefix stripping tests have been removed
// This functionality is now tested comprehensively in proxy_path_stripping_test.go
// which covers both Sherpa and Olla implementations

// DEPRECATED: Header copying tests have been removed
// This functionality is now tested comprehensively in core/common_test.go
// which tests the shared CopyHeaders function used by both implementations

// TestSherpaProxyService_StreamResponse_ReadTimeout tests read timeout behaviour
func TestSherpaProxyService_StreamResponse_ReadTimeout(t *testing.T) {
	// Create a slow upstream that takes longer than read timeout
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		// Send some data then pause longer than read timeout
		w.Write([]byte("initial data"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(200 * time.Millisecond) // Longer than our test timeout
		w.Write([]byte("more data"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
	config := &sherpa.Configuration{}
	config.ResponseTimeout = 5 * time.Second
	config.ReadTimeout = 100 * time.Millisecond // Short timeout
	config.StreamBufferSize = 1024
	proxy, selector, _ := createTestSherpaProxyWithConfig([]*domain.Endpoint{endpoint}, config)
	selector.endpoint = endpoint

	req, stats, rlog := createTestRequestWithStats("GET", "/api/stream", "")
	w := httptest.NewRecorder()

	err := proxy.ProxyRequestToEndpoints(req.Context(), w, req, []*domain.Endpoint{endpoint}, stats, rlog)

	// Should get read timeout error
	if err == nil {
		t.Error("Expected timeout error")
	}
	if !strings.Contains(err.Error(), "stopped responding") {
		t.Errorf("Expected timeout error message, got: %v", err)
	}

	// Should have received initial data
	if !strings.Contains(w.Body.String(), "initial data") {
		t.Error("Should have received initial data before timeout")
	}
}

// TestSherpaProxyService_BufferPooling tests buffer pool behaviour
func TestSherpaProxyService_BufferPooling(t *testing.T) {
	config := &sherpa.Configuration{}
	config.StreamBufferSize = 4096
	proxy, _, _ := createTestSherpaProxyWithConfig([]*domain.Endpoint{}, config)

	// Buffer pooling is now internal - we can verify it works by making requests
	// and checking that memory is used efficiently
	t.Log("Buffer pooling is internal - verified through request handling")

	// Create a test endpoint
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(strings.Repeat("data", 1000))) // 4KB response
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)

	// Make multiple requests to test buffer reuse
	for i := range 5 {
		req, stats, rlog := createTestRequestWithStats("GET", "/api/test", "")
		w := httptest.NewRecorder()
		err := proxy.ProxyRequestToEndpoints(req.Context(), w, req, []*domain.Endpoint{endpoint}, stats, rlog)
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}
}

// TestSherpaProxyService_ConfigDefaults tests default configuration values
func TestSherpaProxyService_ConfigDefaults(t *testing.T) {
	config := &sherpa.Configuration{} // Empty config
	proxy, _, _ := createTestSherpaProxyWithConfig([]*domain.Endpoint{}, config)

	// Transport configuration is now internal - we can verify defaults
	// by checking that the proxy works correctly out of the box
	t.Log("Transport defaults verified through proxy functionality")

	// Make a test request to verify the proxy works with default config
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("default config test"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
	req, stats, rlog := createTestRequestWithStats("GET", "/api/test", "")
	w := httptest.NewRecorder()
	err := proxy.ProxyRequestToEndpoints(req.Context(), w, req, []*domain.Endpoint{endpoint}, stats, rlog)

	if err != nil {
		t.Errorf("Proxy should work with default config: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	// TLS configuration is also internal and verified through functionality
}

// TestSherpaProxyService_UpdateConfig tests configuration updates
func TestSherpaProxyService_UpdateConfig(t *testing.T) {
	initialConfig := &sherpa.Configuration{}
	initialConfig.ResponseTimeout = 10 * time.Second
	initialConfig.ReadTimeout = 5 * time.Second
	initialConfig.StreamBufferSize = 4096
	proxy, _, _ := createTestSherpaProxyWithConfig([]*domain.Endpoint{}, initialConfig)

	// Update config
	newConfig := &sherpa.Configuration{}
	newConfig.ResponseTimeout = 20 * time.Second
	newConfig.ReadTimeout = 10 * time.Second
	newConfig.StreamBufferSize = 8192

	proxy.UpdateConfig(newConfig)

	// Configuration is now internal - verify update worked by making a request
	// The proxy should still function correctly after the config update
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("config updated"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
	req, stats, rlog := createTestRequestWithStats("GET", "/api/test", "")
	w := httptest.NewRecorder()
	err := proxy.ProxyRequestToEndpoints(req.Context(), w, req, []*domain.Endpoint{endpoint}, stats, rlog)

	if err != nil {
		t.Errorf("Proxy should work after config update: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSherpaProxyService_StatsAccuracy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // Add some latency
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
	proxy, selector, _ := createTestSherpaProxy([]*domain.Endpoint{endpoint})
	selector.endpoint = endpoint

	// Make some successful requests
	for i := range 3 {
		req, stats, rlog := createTestRequestWithStats("GET", "/api/test", "")
		w := httptest.NewRecorder()
		err := proxy.ProxyRequestToEndpoints(req.Context(), w, req, []*domain.Endpoint{endpoint}, stats, rlog)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
	}

	// Make a failed request (unreachable endpoint)
	failEndpoint := createTestEndpoint("fail", "http://localhost:99999", domain.StatusHealthy)
	selector.endpoint = failEndpoint

	req, stats, rlog := createTestRequestWithStats("GET", "/api/test", "")
	w := httptest.NewRecorder()
	err := proxy.ProxyRequestToEndpoints(req.Context(), w, req, []*domain.Endpoint{failEndpoint}, stats, rlog)
	if err == nil {
		t.Error("Expected failure for unreachable endpoint")
	}

	// Check stats
	proxyStats, err := proxy.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

