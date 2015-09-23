package security

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

func createTestMetricsLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}
func createNewTestSecurityMetricsAdapter() *MetricsAdapter {
	kennyloggins := createTestMetricsLogger()
	statsCollector := createTestStatsCollector(kennyloggins)
	return NewSecurityMetricsAdapter(statsCollector, kennyloggins)
}
func TestNewSecurityMetricsAdapter(t *testing.T) {
	adapter := createNewTestSecurityMetricsAdapter()

	if adapter == nil {
		t.Fatal("NewSecurityMetricsAdapter returned nil")
	}
	if adapter.statsCollector == nil {
		t.Error("statsCollector not set")
	}
	if adapter.logger == nil {
		t.Error("Logger not set")
	}
}

func TestSecurityMetricsAdapter_RecordViltion_RateLimit(t *testing.T) {
	adapter := createNewTestSecurityMetricsAdapter()
	ctx := context.Background()

	violation := ports.SecurityViolation{
		ClientID:      "192.168.1.100",
		ViolationType: "rate_limit",
		Endpoint:      "/api/test",
		Size:          0,
		Timestamp:     time.Now(),
	}

	err := adapter.RecordViolation(ctx, violation)
	if err != nil {
		t.Fatalf("RecordViolation failed: %v", err)
	}

	metrics, err := adapter.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if metrics.RateLimitViolations != 1 {
		t.Errorf("Expected 1 rate limit violation, got %d", metrics.RateLimitViolations)
	}
	if metrics.UniqueRateLimitedIPs != 1 {
		t.Errorf("Expected 1 unique rate limited IP, got %d", metrics.UniqueRateLimitedIPs)
	}
	if metrics.SizeLimitViolations != 0 {
		t.Errorf("Expected 0 size limit violations, got %d", metrics.SizeLimitViolations)
	}
}

func TestSecurityMetricsAdapter_RecordViolation_LargeRequest(t *testing.T) {
	adapter := createNewTestSecurityMetricsAdapter()
	ctx := context.Background()

	violation := ports.SecurityViolation{
		ClientID:      "192.168.1.100",
		ViolationType: "size_limit",
		Endpoint:      "/api/test",
		Size:          100 * 1024 * 1024,
		Timestamp:     time.Now(),
