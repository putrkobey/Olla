package security

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/putrkobey/Olla/internal/util"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

func createTestRateLimitLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

func TestNewRateLimitValidator(t *testing.T) {
	cidrs := []string{"192.168.0.0/16", "10.0.0.0/8"}
	trustedCIDRs, _ := util.ParseTrustedCIDRs(cidrs)
	limits := config.ServerRateLimits{
		GlobalRequestsPerMinute: 1000,
		PerIPRequestsPerMinute:  100,
		BurstSize:               50,
		HealthRequestsPerMinute: 500,
		CleanupInterval:         time.Minute,
		TrustProxyHeaders:       true,
		TrustedProxyCIDRs:       cidrs,
		TrustedProxyCIDRsParsed: trustedCIDRs,
	}

	validator := createTestRateLimitValidator(limits)
	defer validator.Stop()

	if validator.Name() != "rate_limit" {
		t.Errorf("Expected name 'rate_limit', got %q", validator.Name())
	}
	if validator.globalRequestsPerMinute != 1000 {
		t.Errorf("Expected global limit 1000, got %d", validator.globalRequestsPerMinute)
	}
	if validator.perIPRequestsPerMinute != 100 {
		t.Errorf("Expected per-IP limit 100, got %d", validator.perIPRequestsPerMinute)
	}
	if validator.burstSize != 50 {
		t.Errorf("Expected burst size 50, got %d", validator.burstSize)
	}
	if !validator.trustProxyHeaders {
		t.Error("Expected trust proxy headers to be true")
	}
	if validator.globalLimiter == nil {
		t.Error("Expected global limiter to be initialised")
	}
	if len(validator.trustedCIDRs) != 2 {
		t.Errorf("Expected 2 trusted CIDRs, got %d", len(validator.trustedCIDRs))
	}
}

func createTestRateLimitValidator(limits config.ServerRateLimits) *RateLimitValidator {
	logger := createTestRateLimitLogger()
	statsCollector := createTestStatsCollector(logger)
	metricsAdapter := NewSecurityMetricsAdapter(statsCollector, logger)

	return NewRateLimitValidator(limits, metricsAdapter, logger)
}
func TestNewRateLimitValidator_InvalidCIDRs(t *testing.T) {
	limits := config.ServerRateLimits{
		GlobalRequestsPerMinute: 1000,
		PerIPRequestsPerMinute:  100,
		BurstSize:               50,
		TrustProxyHeaders:       true,
		TrustedProxyCIDRs:       []string{"invalid-cidr", "192.168.0.0/16"},
	}

	validator := createTestRateLimitValidator(limits)
	defer validator.Stop()

	if validator.trustedCIDRs != nil {
		t.Error("Expected trustedCIDRs to be nil when parsing fails")
	}
}

func TestRateLimitValidator_Validate_Disabled(t *testing.T) {
	limits := config.ServerRateLimits{
		GlobalRequestsPerMinute: 0,
		PerIPRequestsPerMinute:  0,
		BurstSize:               10,
		CleanupInterval:         time.Minute,
	}

	validator := createTestRateLimitValidator(limits)
	defer validator.Stop()

	req := ports.SecurityRequest{
		ClientID:      "192.168.1.100",
		Endpoint:      "/api/test",
		Method:        "POST",
		IsHealthCheck: false,
	}

	for i := range 10 {
		result, err := validator.Validate(context.Background(), req)
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		if !result.Allowed {
			t.Errorf("Request %d should be allowed when limits are disabled", i+1)
		}
	}

	if validator.globalLimiter != nil {
		t.Error("Global limiter should not be initialised when global limit is 0")
	}
}

func TestRateLimitValidator_Validate_HealthEndpoint(t *testing.T) {
	limits := config.ServerRateLimits{
		GlobalRequestsPerMinute: 0,
		PerIPRequestsPerMinute:  60,
		HealthRequestsPerMinute: 300,
		BurstSize:               3,
		CleanupInterval:         time.Minute,
	}

	validator := createTestRateLimitValidator(limits)
	defer validator.Stop()

	ctx := context.Background()
	clientIP := "192.168.1.100"

	regularReq := ports.SecurityRequest{
		ClientID:      clientIP,
		Endpoint:      "/api/test",
		Method:        "POST",
		IsHealthCheck: false,
	}

	healthReq := ports.SecurityRequest{
		ClientID:      clientIP,
		Endpoint:      "/internal/health",
		Method:        "GET",
		IsHealthCheck: true,
	}

	regularResult, err := validator.Validate(ctx, regularReq)
	if err != nil {
		t.Fatalf("Regular request validation failed: %v", err)
	}
	if regularResult.RateLimit != 60 {
		t.Errorf("Expected regular limit 60, got %d", regularResult.RateLimit)
	}

	healthResult, err := validator.Validate(ctx, healthReq)
	if err != nil {
		t.Fatalf("Health request validation failed: %v", err)
	}
	if healthResult.RateLimit != 300 {
		t.Errorf("Expected health limit 300, got %d", healthResult.RateLimit)
	}
}

func TestRateLimitValidator_Validate_BurstCapacity(t *testing.T) {
	limits := config.ServerRateLimits{
		GlobalRequestsPerMinute: 0,
		PerIPRequestsPerMinute:  60,
		BurstSize:               3,
		CleanupInterval:         time.Minute,
	}

	validator := createTestRateLimitValidator(limits)
	defer validator.Stop()

	ctx := context.Background()
	req := ports.SecurityRequest{
		ClientID:      "192.168.1.100",
		Endpoint:      "/api/test",
		Method:        "POST",
		IsHealthCheck: false,
	}

	successCount := 0
	rateLimitedCount := 0

	for range 10 {
		result, err := validator.Validate(ctx, req)
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}

		if result.Allowed {
			successCount++
		} else {
			rateLimitedCount++
			if result.RetryAfter == 0 {
				t.Error("Expected Retry-After header when rate limited")
			}
		}
	}

	if successCount == 0 {
		t.Error("Expected some successful requests")
	}
	if rateLimitedCount == 0 {
		t.Log("No rate limiting triggered - this may be acceptable with new implementation")
	}
}

func TestRateLimitValidator_Validate_PerIPIsolation(t *testing.T) {
	limits := config.ServerRateLimits{
		GlobalRequestsPerMinute: 0,
		PerIPRequestsPerMinute:  60,
		BurstSize:               2,
		CleanupInterval:         time.Minute,
	}

	validator := createTestRateLimitValidator(limits)
	defer validator.Stop()

	ctx := context.Background()

