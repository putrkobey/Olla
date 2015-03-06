package security

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

func createTestSizeLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}
func createTestSizeLimitValidator(limits config.ServerRequestLimits) *SizeValidator {
	logger := createTestRateLimitLogger()
	statsCollector := createTestStatsCollector(logger)
	metricsAdapter := NewSecurityMetricsAdapter(statsCollector, logger)

	return NewSizeValidator(limits, metricsAdapter, logger)
}
func TestNewSizeValidator(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   1024,
		MaxHeaderSize: 512,
	}

	validator := createTestSizeLimitValidator(limits)

	if validator.Name() != "size_limit" {
		t.Errorf("Expected name 'size_limit', got %q", validator.Name())
	}
	if validator.maxBodySize != 1024 {
		t.Errorf("Expected max body size 1024, got %d", validator.maxBodySize)
	}
	if validator.maxHeaderSize != minHeaderSize {
		t.Errorf("Expected max header size floored to %d, got %d", minHeaderSize, validator.maxHeaderSize)
	}
}

func TestNewSizeValidator_HeaderSizeFloorAndDefault(t *testing.T) {
	tests := []struct {
		name       string
		configured int64
		want       int64
	}{
		{"zero falls back to the default cap", 0, http.DefaultMaxHeaderBytes},
		{"negative falls back to the default cap", -100, http.DefaultMaxHeaderBytes},
		{"small positive value is floored", 100, minHeaderSize},
		{"exactly the floor stays unchanged", minHeaderSize, minHeaderSize},
		{"large explicit value is respected", 2 * 1024 * 1024, 2 * 1024 * 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := createTestSizeLimitValidator(config.ServerRequestLimits{MaxHeaderSize: tt.configured})
			if validator.maxHeaderSize != tt.want {
				t.Errorf("effectiveMaxHeaderSize(%d) = %d, want %d", tt.configured, validator.maxHeaderSize, tt.want)
			}
		})
	}
}

func TestSizeValidator_Validate_SmallRequest(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   1024,
		MaxHeaderSize: 512,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   100,
		HeaderSize: 200,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"User-Agent":   {"TestAgent/1.0"},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Allowed {
		t.Errorf("Small request should be allowed, got: %s", result.Reason)
	}
}

func TestSizeValidator_Validate_BodyTooLarge(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   100,
		MaxHeaderSize: 1024,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   200,
		HeaderSize: 50,
		Headers:    map[string][]string{},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Allowed {
		t.Error("Large body should be rejected")
	}
	if !strings.Contains(result.Reason, "Request body too large") {
		t.Errorf("Expected body size error, got: %s", result.Reason)
	}
}

func TestSizeValidator_Validate_HeadersTooLarge(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   1024,
		MaxHeaderSize: minHeaderSize,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   50,
		HeaderSize: minHeaderSize + 500,
		Headers: map[string][]string{
			"X-Large-Header": {strings.Repeat("x", minHeaderSize+500)},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Allowed {
		t.Error("Large headers should be rejected")
	}
	if !strings.Contains(result.Reason, "Request headers too large") {
		t.Errorf("Expected header size error, got: %s", result.Reason)
	}
}

// TestSizeValidator_Validate_ZeroHeaderLimitUsesDefaultCap confirms max_header_size:
// 0 no longer disables header validation - it falls back to the default 1 MiB
// cap (matching the stdlib http.Server fallback), so a header block beyond
// that default is still rejected even though the operator wrote "0".
func TestSizeValidator_Validate_ZeroHeaderLimitUsesDefaultCap(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   0,
		MaxHeaderSize: 0,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	oversized := strings.Repeat("z", http.DefaultMaxHeaderBytes+1)
	req := ports.SecurityRequest{
		ClientID: "192.168.1.100",
		Endpoint: "/api/test",
		Method:   "POST",
		BodySize: 10000,
		Headers: map[string][]string{
			"X-Huge-Header": {oversized},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Allowed {
		t.Error("a header block over the default 1 MiB cap must be rejected even when max_header_size is 0")
	}

	// Body validation is genuinely disabled at 0 - unaffected by this change.
	req.Headers = map[string][]string{"Content-Type": {"application/json"}}
	result, err = validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Allowed {
		t.Errorf("body size should remain unbounded when max_body_size is 0, got: %s", result.Reason)
	}
}

// TestSizeValidator_Validate_ZeroBodyDisabled_ZeroHeaderUnderDefaultCap
// covers max_body_size: 0 and max_header_size: 0 together at small values
// that pass either way, so it doesn't overlap with
// TestSizeValidator_Validate_ZeroHeaderLimitUsesDefaultCap (which asserts
// the header default cap actually rejects an oversized block). The two
// zeros mean different things: max_body_size: 0 genuinely disables the body
// check (unbounded), but max_header_size: 0 is NOT "disabled" - it falls
// back to the default 1 MiB cap - these headers are just small enough
// (5000 bytes) to pass under that cap regardless, not because the check is
// off.
func TestSizeValidator_Validate_ZeroBodyDisabled_ZeroHeaderUnderDefaultCap(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   0,
		MaxHeaderSize: 0,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   10000,
		HeaderSize: 5000,
		Headers: map[string][]string{
			"X-Huge-Header": {strings.Repeat("z", 5000)},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected the body (disabled) and small headers (under the default cap) to both be allowed, got: %s", result.Reason)
	}
}

func TestSizeValidator_Validate_NegativeLimitsDisabled(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   -1,
		MaxHeaderSize: -100,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   5000,
		HeaderSize: 2000,
		Headers: map[string][]string{
			"X-Large-Header": {strings.Repeat("z", 2000)},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Allowed {
		t.Errorf("Request should be allowed when limits are negative (disabled), got: %s", result.Reason)
	}
}

func TestSizeValidator_Validate_ExactLimit(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   100,
		MaxHeaderSize: 200,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   100,
		HeaderSize: 150,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Allowed {
		t.Errorf("Request at exact limit should be allowed, got: %s", result.Reason)
	}
}

func TestSizeValidator_Validate_OneByteTooLarge(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   100,
		MaxHeaderSize: 200,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   101,
		HeaderSize: 150,
		Headers:    map[string][]string{},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Allowed {
		t.Error("Request one byte over limit should be rejected")
	}
}

func TestSizeValidator_Validate_EmptyBody(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   100,
		MaxHeaderSize: 200,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "GET",
		BodySize:   0,
		HeaderSize: 50,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !result.Allowed {
		t.Errorf("Empty body should be allowed, got: %s", result.Reason)
	}
}

func TestSizeValidator_Validate_MultipleHeaders(t *testing.T) {
	limits := config.ServerRequestLimits{
		MaxBodySize:   1024,
		MaxHeaderSize: minHeaderSize,
	}

	validator := createTestSizeLimitValidator(limits)
	ctx := context.Background()

	req := ports.SecurityRequest{
		ClientID:   "192.168.1.100",
		Endpoint:   "/api/test",
		Method:     "POST",
		BodySize:   100,
		HeaderSize: minHeaderSize + 300,
		Headers: map[string][]string{
			"Content-Type":    {"application/json"},
			"Authorization":   {"Bearer " + strings.Repeat("x", minHeaderSize)},
			"X-Custom-Header": {strings.Repeat("y", 50)},
			"User-Agent":      {"TestAgent/1.0"},
		},
	}

	result, err := validator.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if result.Allowed {
