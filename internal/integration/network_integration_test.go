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
