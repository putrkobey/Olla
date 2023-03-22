package services

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

func newTestLogger() logger.StyledLogger {
	cfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(cfg)
	return logger.NewPlainStyledLogger(log)
}

// spyProxyService is a minimal ports.ProxyService that also implements the
// cleanupable interface. It counts Cleanup invocations for assertion.
type spyProxyService struct {
	cleanupCalls atomic.Int32
}

func (s *spyProxyService) ProxyRequest(_ context.Context, _ http.ResponseWriter, _ *http.Request, _ *ports.RequestStats, _ logger.StyledLogger) error {
	return nil
}

func (s *spyProxyService) ProxyRequestToEndpoints(_ context.Context, _ http.ResponseWriter, _ *http.Request, _ []*domain.Endpoint, _ *ports.RequestStats, _ logger.StyledLogger) error {
	return nil
}

func (s *spyProxyService) GetStats(_ context.Context) (ports.ProxyStats, error) {
	return ports.ProxyStats{}, nil
}

func (s *spyProxyService) UpdateConfig(_ ports.ProxyConfiguration) {}

