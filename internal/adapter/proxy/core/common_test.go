package core

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestCopyHeaders(t *testing.T) {
	tests := []struct {
		name            string
		originalHeaders map[string][]string
		expectedCopied  map[string][]string
		expectedSkipped []string
	}{
		{
			name: "basic_headers_copied",
			originalHeaders: map[string][]string{
				"Content-Type":    {"application/json"},
				"Accept":          {"text/html"},
				"User-Agent":      {"test-agent"},
				"X-Custom-Header": {"custom-value"},
			},
			expectedCopied: map[string][]string{
				"Content-Type":    {"application/json"},
				"Accept":          {"text/html"},
				"User-Agent":      {"test-agent"},
				"X-Custom-Header": {"custom-value"},
			},
			expectedSkipped: []string{},
		},
		{
			name: "security_headers_filtered",
			originalHeaders: map[string][]string{
				"Content-Type":        {"application/json"},
				"Authorization":       {"Bearer secret-token"},
				"Cookie":              {"session=secret"},
				"X-Api-Key":           {"secret-api-key"},
				"X-Auth-Token":        {"secret-auth-token"},
				"Proxy-Authorization": {"Basic secret"},
			},
			expectedCopied: map[string][]string{
				"Content-Type": {"application/json"},
			},
			expectedSkipped: []string{
				"Authorization",
				"Cookie",
				"X-Api-Key",
				"X-Auth-Token",
				"Proxy-Authorization",
			},
		},
		{
			name: "hop_by_hop_headers_filtered",
			originalHeaders: map[string][]string{
				"Content-Type":        {"application/json"},
				"Connection":          {"keep-alive"},
				"Keep-Alive":          {"timeout=5"},
				"Proxy-Authenticate":  {"Basic"},
				"Proxy-Authorization": {"Basic secret"},
				"TE":                  {"trailers"},
				"Trailer":             {"X-Custom"},
				"Transfer-Encoding":   {"chunked"},
				"Upgrade":             {"websocket"},
			},
			expectedCopied: map[string][]string{
				"Content-Type": {"application/json"},
			},
			expectedSkipped: []string{
				"Connection",
				"Keep-Alive",
				"Proxy-Authenticate",
				"Proxy-Authorization",
				"TE",
				"Trailer",
				"Transfer-Encoding",
				"Upgrade",
			},
		},
		{
			name: "multi_value_headers",
			originalHeaders: map[string][]string{
				"Accept":     {"text/html", "application/json"},
				"X-Custom":   {"value1", "value2", "value3"},
				"Set-Cookie": {"cookie1=value1", "cookie2=value2"},
			},
			expectedCopied: map[string][]string{
				"Accept":     {"text/html", "application/json"},
				"X-Custom":   {"value1", "value2", "value3"},
				"Set-Cookie": {"cookie1=value1", "cookie2=value2"},
			},
			expectedSkipped: []string{},
		},
		{
			name:            "empty_headers",
			originalHeaders: map[string][]string{},
			expectedCopied:  map[string][]string{},
			expectedSkipped: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create original request with headers
			originalReq := httptest.NewRequest("GET", "http://example.com/test", nil)
			for k, values := range tt.originalHeaders {
				for _, v := range values {
					originalReq.Header.Add(k, v)
				}
			}

			// Create proxy request
			proxyReq := httptest.NewRequest("GET", "http://backend.com/test", nil)

			// Copy headers
			CopyHeaders(proxyReq, originalReq, nil)

			// Check that expected headers were copied
			for k, expectedValues := range tt.expectedCopied {
				actualValues := proxyReq.Header[k]
				assert.Equal(t, expectedValues, actualValues, "Header %s should be copied correctly", k)
			}

			// Check that security headers were NOT copied
			for _, skippedHeader := range tt.expectedSkipped {
				assert.Empty(t, proxyReq.Header.Get(skippedHeader), "Header %s should not be copied", skippedHeader)
			}

			// Check proxy-specific headers are added
			assert.NotEmpty(t, proxyReq.Header.Get("X-Proxied-By"))
			assert.NotEmpty(t, proxyReq.Header.Get("Via"))
		})
	}
}

func TestCopyHeaders_ProxyHeaders(t *testing.T) {
	tests := []struct {
		name                   string
		originalHeaders        map[string][]string
		remoteAddr             string
		tls                    bool
		expectedForwardedFor   string
		expectedForwardedProto string
		expectedForwardedHost  string
		expectedRealIP         string
		expectedVia            string
	}{
		{
			name:                   "http_request_basic",
			originalHeaders:        map[string][]string{},
			remoteAddr:             "192.168.1.100:12345",
			tls:                    false,
			expectedForwardedFor:   "192.168.1.100",
			expectedForwardedProto: constants.ProtocolHTTP,
			expectedForwardedHost:  "example.com",
			expectedRealIP:         "192.168.1.100",
		},
		{
			name:                   "https_request_basic",
			originalHeaders:        map[string][]string{},
			remoteAddr:             "192.168.1.100:12345",
			tls:                    true,
			expectedForwardedFor:   "192.168.1.100",
			expectedForwardedProto: constants.ProtocolHTTPS,
			expectedForwardedHost:  "example.com",
			expectedRealIP:         "192.168.1.100",
		},
		{
			name: "existing_forwarded_headers",
			originalHeaders: map[string][]string{
				"X-Forwarded-For":   {"10.0.0.1"},
				"X-Forwarded-Proto": {"https"},
				"X-Forwarded-Host":  {"original.com"},
				"X-Real-IP":         {"10.0.0.1"},
			},
			remoteAddr:             "192.168.1.100:12345",
			tls:                    false,
			expectedForwardedFor:   "10.0.0.1, 10.0.0.1", // extractClientIP will get from X-Forwarded-For
			expectedForwardedProto: "https",              // Preserves existing
			expectedForwardedHost:  "original.com",       // Preserves existing
			expectedRealIP:         "10.0.0.1",           // Preserves existing
		},
		{
			name: "via_header_chaining",
			originalHeaders: map[string][]string{
				"Via": {"1.1 proxy1"},
			},
			remoteAddr:  "192.168.1.100:12345",
			tls:         false,
			expectedVia: "1.1 proxy1, " + GetViaHeader(),
		},
		{
			name:                   "malformed_remote_addr",
			originalHeaders:        map[string][]string{},
			remoteAddr:             "invalid-addr",
			tls:                    false,
			expectedForwardedFor:   "", // Should not be set due to error
			expectedForwardedProto: constants.ProtocolHTTP,
			expectedForwardedHost:  "example.com",
			expectedRealIP:         "", // Should not be set due to error
		},
	}
