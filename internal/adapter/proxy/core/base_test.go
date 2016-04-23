package core

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func createBaseTestLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

func newBaseTestComponents() (*BaseProxyComponents, *stats.Collector) {
	collector := stats.NewCollector(createBaseTestLogger())
	return &BaseProxyComponents{StatsCollector: collector}, collector
}

func newBaseTestEndpoint() *domain.Endpoint {
	u, _ := url.Parse("http://localhost:11434")
	return &domain.Endpoint{
		Name:      "ep1",
		URL:       u,
		URLString: u.String(),
		Status:    domain.StatusHealthy,
	}
}

// TestBaseProxyComponents_RecordSuccess_WithModelName is the regression test
// for RecordModelRequest's dead-since-ed93e9a callers: a successful request
// with a resolved model name must populate the model-level stats, not just
// the per-endpoint ones, so /internal/stats/models has data to report.
func TestBaseProxyComponents_RecordSuccess_WithModelName(t *testing.T) {
	b, collector := newBaseTestComponents()
	endpoint := newBaseTestEndpoint()

	b.RecordSuccess(endpoint, "llama3.2", 42, 1024)

	modelStats := collector.GetModelStats()
	ms, ok := modelStats["llama3.2"]
	if !ok {
		t.Fatalf("expected model stats for llama3.2, got %v", modelStats)
	}
	if ms.TotalRequests != 1 || ms.SuccessfulRequests != 1 {
		t.Errorf("expected 1 total/successful request, got %+v", ms)
	}
