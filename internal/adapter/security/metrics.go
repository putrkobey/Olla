package security

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

type MetricsAdapter struct {
	statsCollector ports.StatsCollector
	logger         logger.StyledLogger
}

// NewSecurityMetricsAdapter concise way to capture security metrics for now
func NewSecurityMetricsAdapter(statsCollector ports.StatsCollector, logger logger.StyledLogger) *MetricsAdapter {
	return &MetricsAdapter{
		statsCollector: statsCollector,
		logger:         logger,
