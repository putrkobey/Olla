package services

import (
	"context"
	"errors"

	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// StatsService provides centralised metrics collection for all system components.
// It initialises early in the startup sequence as most other services depend on it
// for instrumentation and observability.
type StatsService struct {
	collector ports.StatsCollector
	logger    logger.StyledLogger
}

// NewStatsService creates a new stats service
func NewStatsService(logger logger.StyledLogger) *StatsService {
	return &StatsService{
		logger: logger,
	}
}

// Name returns the service name
func (s *StatsService) Name() string {
	return "stats"
}
