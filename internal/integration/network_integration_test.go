package integration

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/putrkobey/Olla/internal/util"

	"github.com/putrkobey/Olla/internal/config"
)

func TestTrustedProxyCIDRsIntegration(t *testing.T) {
	// Test the full integration with config loading and IP extraction because this is one
	// chunky monkey and tricky ricky to get right with all the edge cases and importance!
	cfg := &config.Config{
		Server: config.ServerConfig{
			RateLimits: config.ServerRateLimits{
				TrustProxyHeaders: true,
				TrustedProxyCIDRs: []string{
					"127.0.0.0/8",
					"10.0.0.0/8",
					"172.16.0.0/12",
					"192.168.0.0/16",
				},
			},
		},
	}

	trustedCIDRs, err := util.ParseTrustedCIDRs(cfg.Server.RateLimits.TrustedProxyCIDRs)
	if err != nil {
		t.Fatalf("Failed to parse trusted CIDRs: %v", err)
	}

	testCases := []struct {
		name          string
		remoteAddr    string
		xForwardedFor string
		xRealIP       string
		expectedIP    string
		description   string
	}{
		{
			name:          "trusted_proxy_with_x_forwarded_for",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "203.0.113.1, 198.51.100.1",
			expectedIP:    "203.0.113.1",
			description:   "Should use first IP from X-Forwarded-For when coming from trusted proxy",
		},
		{
			name:          "untrusted_proxy_with_x_forwarded_for",
			remoteAddr:    "203.0.113.1:12345",
			xForwardedFor: "10.0.0.1",
			expectedIP:    "203.0.113.1",
			description:   "Should ignore X-Forwarded-For from untrusted source",
		},
		{
			name:        "trusted_proxy_with_x_real_ip",
			remoteAddr:  "10.0.0.1:12345",
			xRealIP:     "203.0.113.50",
			expectedIP:  "203.0.113.50",
			description: "Should use X-Real-IP when coming from trusted proxy",
		},
		{
			name:        "no_proxy_headers",
			remoteAddr:  "203.0.113.1:12345",
			expectedIP:  "203.0.113.1",
			description: "Should use remote address when no proxy headers",
		},
		{
			name:          "localhost_proxy",
			remoteAddr:    "127.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			expectedIP:    "203.0.113.1",
			description:   "Should trust localhost as proxy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tc.remoteAddr

			if tc.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.xForwardedFor)
			}
			if tc.xRealIP != "" {
				req.Header.Set("X-Real-IP", tc.xRealIP)
			}

			ip := util.GetClientIP(req, cfg.Server.RateLimits.TrustProxyHeaders, trustedCIDRs)
			if ip != tc.expectedIP {
				t.Errorf("%s: expected %s, got %s", tc.description, tc.expectedIP, ip)
			}
		})
	}
}

func TestConfigDefaultCIDRsIntegration(t *testing.T) {
	// Test that default config CIDRs parse correctly
	cfg := config.DefaultConfig()

	trustedCIDRs, err := util.ParseTrustedCIDRs(cfg.Server.RateLimits.TrustedProxyCIDRs)
	if err != nil {
		t.Fatalf("Default config CIDRs should parse without error: %v", err)
	}

	if len(trustedCIDRs) != 4 {
		t.Errorf("Expected 4 default CIDRs, got %d", len(trustedCIDRs))
	}

	// Test that common private IPs are in the trusted ranges
	commonPrivateIPs := []string{
		"127.0.0.1",   // localhost
		"10.0.0.1",    // private class A
		"172.16.0.1",  // private class B
		"192.168.1.1", // private class C
	}

	for _, ipStr := range commonPrivateIPs {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ipStr + ":12345"
