package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// TestMakeUserFriendlyError tests the error handling utility function
func TestMakeUserFriendlyError(t *testing.T) {
	timeout := 30 * time.Second

	testCases := []struct {
		name                string
		inputError          error
		duration            time.Duration
		context             string
		expectedContains    []string
		notExpectedContains []string
	}{
		{
			name:             "Context Canceled",
			inputError:       context.Canceled,
			duration:         5 * time.Second,
			context:          "streaming",
			expectedContains: []string{"request cancelled", "client disconnected early", "5.0s"},
		},
		{
			name:             "Context Deadline Exceeded",
			inputError:       context.DeadlineExceeded,
			duration:         30 * time.Second,
			context:          "backend",
			expectedContains: []string{"request timeout", "30.0s", "server timeout", "exceeded"},
		},
		{
			name:             "Connection Refused",
			inputError:       &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			duration:         2 * time.Second,
			context:          "backend",
			expectedContains: []string{"network error", "connection refused", "2.0s"},
		},
		{
			name:             "Connection Reset",
			inputError:       errors.New("connection reset by peer"),
			duration:         1 * time.Second,
			context:          "streaming",
			expectedContains: []string{"connection reset", "1.0s", "closed connection unexpectedly"},
		},
		{
			name:             "DNS Resolution Error",
			inputError:       errors.New("no such host"),
			duration:         3 * time.Second,
			context:          "backend",
			expectedContains: []string{"DNS lookup failed", "3.0s", "cannot resolve LLM backend hostname"},
		},
		{
			name:             "EOF Error",
			inputError:       io.EOF,
			duration:         10 * time.Second,
			context:          "streaming",
			expectedContains: []string{"AI backend closed connection", "10.0s", "response stream ended unexpectedly"},
		},
		{
			name:             "Network Unreachable",
			inputError:       errors.New("network is unreachable"),
			duration:         5 * time.Second,
			context:          "backend",
			expectedContains: []string{"request failed", "5.0s", "network is unreachable"},
		},
		{
			name:             "Generic Error",
			inputError:       errors.New("some unexpected error"),
			duration:         1500 * time.Millisecond,
			context:          "selection",
			expectedContains: []string{"some unexpected error", "1.5s"},
		},
		{
			name:                "Very Short Duration",
			inputError:          context.DeadlineExceeded,
			duration:            100 * time.Millisecond,
			context:             "backend",
			expectedContains:    []string{"request timeout", "0.1s", "server timeout"},
			notExpectedContains: []string{"after 0.0s"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MakeUserFriendlyError(tc.inputError, tc.duration, tc.context, timeout)

			for _, expected := range tc.expectedContains {
				if !strings.Contains(result.Error(), expected) {
					t.Errorf("Expected error to contain %q, got: %s", expected, result.Error())
				}
			}

			for _, notExpected := range tc.notExpectedContains {
				if strings.Contains(result.Error(), notExpected) {
					t.Errorf("Expected error NOT to contain %q, got: %s", notExpected, result.Error())
				}
			}
		})
	}
}

// TestProxyErrorCreation tests domain.NewProxyError functionality
func TestProxyErrorCreation(t *testing.T) {
	requestID := "test-req-123"
	targetURL := "http://example.com/api/test"
	method := "POST"
	path := "/api/test"
	statusCode := 500
	duration := 2500 * time.Millisecond
	bytes := int64(1024)
	cause := errors.New("upstream server error")

	proxyErr := domain.NewProxyError(requestID, targetURL, method, path, statusCode, duration, int(bytes), cause)

	if proxyErr == nil {
		t.Fatal("NewProxyError should not return nil")
	}

	errorStr := proxyErr.Error()

	// Check that the error contains key information
	expectedContains := []string{
		requestID,
		method,
		path,
		"2.5s",
		"upstream server error",
	}

	for _, expected := range expectedContains {
		if !strings.Contains(errorStr, expected) {
			t.Errorf("Expected error to contain %q, got: %s", expected, errorStr)
		}
	}
}

// TestProxyImplementationParity tests that both implementations behave identically
func TestProxyImplementationParity(t *testing.T) {
	testCases := []struct {
		name        string
		setupServer func() *httptest.Server
		setupConfig func() (sherpaConfig *sherpa.Configuration, ollaConfig *olla.Configuration)
		testRequest func(proxy ports.ProxyService, endpoint *domain.Endpoint) error
		expectError bool
		errorCheck  func(err error) bool
	}{
		{
			name: "Successful Simple Request",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "ok"}`))
				}))
			},
			setupConfig: func() (*sherpa.Configuration, *olla.Configuration) {
				sherpaConfig := &sherpa.Configuration{}
				sherpaConfig.ResponseTimeout = 30 * time.Second
				sherpaConfig.ReadTimeout = 10 * time.Second
				sherpaConfig.StreamBufferSize = 8192

				ollaConfig := &olla.Configuration{}
				ollaConfig.ResponseTimeout = 30 * time.Second
				ollaConfig.ReadTimeout = 10 * time.Second
				ollaConfig.StreamBufferSize = 8192
				ollaConfig.MaxIdleConns = 200
				ollaConfig.IdleConnTimeout = 90 * time.Second
				ollaConfig.MaxConnsPerHost = 50
				return sherpaConfig, ollaConfig
			},
			testRequest: func(proxy ports.ProxyService, endpoint *domain.Endpoint) error {
				req, stats, rlog := createTestRequestWithBody("GET", "/api/test", "")
				w := httptest.NewRecorder()
				return proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
			},
			expectError: false,
		},
		{
			name: "Connection Refused Error",
			setupServer: func() *httptest.Server {
				return nil // Will use unreachable endpoint
			},
			setupConfig: func() (*sherpa.Configuration, *olla.Configuration) {
				sherpaConfig := &sherpa.Configuration{}
				sherpaConfig.ResponseTimeout = 5 * time.Second
				sherpaConfig.ReadTimeout = 2 * time.Second
				sherpaConfig.StreamBufferSize = 8192

				ollaConfig := &olla.Configuration{}
				ollaConfig.ResponseTimeout = 5 * time.Second
				ollaConfig.ReadTimeout = 2 * time.Second
				ollaConfig.StreamBufferSize = 8192
				ollaConfig.MaxIdleConns = 200
				ollaConfig.IdleConnTimeout = 90 * time.Second
				ollaConfig.MaxConnsPerHost = 50
				return sherpaConfig, ollaConfig
			},
			testRequest: func(proxy ports.ProxyService, endpoint *domain.Endpoint) error {
				req, stats, rlog := createTestRequestWithBody("GET", "/api/test", "")
				w := httptest.NewRecorder()
				return proxy.ProxyRequest(req.Context(), w, req, stats, rlog)
			},
			expectError: true,
			errorCheck: func(err error) bool {
				return strings.Contains(strings.ToLower(err.Error()), "connection") ||
					strings.Contains(strings.ToLower(err.Error()), "refused") ||
					strings.Contains(strings.ToLower(err.Error()), "dial")
			},
		},
		{
			name: "Streaming Response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(http.StatusOK)
					flusher := w.(http.Flusher)
					for i := range 5 {
						fmt.Fprintf(w, "chunk %d\n", i)
						flusher.Flush()
						time.Sleep(10 * time.Millisecond)
					}
				}))
			},
			setupConfig: func() (*sherpa.Configuration, *olla.Configuration) {
				sherpaConfig := &sherpa.Configuration{}
				sherpaConfig.ResponseTimeout = 30 * time.Second
				sherpaConfig.ReadTimeout = 10 * time.Second
				sherpaConfig.StreamBufferSize = 1024

