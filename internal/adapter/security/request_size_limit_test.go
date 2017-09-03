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
