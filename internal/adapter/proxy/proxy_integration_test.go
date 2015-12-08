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
