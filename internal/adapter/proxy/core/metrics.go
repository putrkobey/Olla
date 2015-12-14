package core

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// ExtractProviderMetrics extracts metrics from the last chunk of a response.
// This is a common operation for both proxy engines.
// Optimized to minimize hot path impact:
// - Early returns for common cases
// - Only logs at debug level
// - Extraction happens with timeout protection in the extractor
func ExtractProviderMetrics(
	ctx context.Context,
	extractor ports.MetricsExtractor,
	lastChunk []byte,
	endpoint *domain.Endpoint,
	stats *ports.RequestStats,
	rlog logger.StyledLogger,
	engineName string,
) {
	// Early return if we can't extract metrics - most common case first
	if extractor == nil || len(lastChunk) == 0 || endpoint == nil || endpoint.Type == "" {
		// Skip logging in hot path unless needed for debugging
		return
	}

	// Skip extraction if context is already cancelled
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Attempt extraction - this has its own timeout protection
	stats.ProviderMetrics = extractor.ExtractFromChunk(ctx, lastChunk, endpoint.Type)

	// Minimal debug logging for metrics extraction results
	// The logger implementation will handle whether to actually log based on level
	if stats.ProviderMetrics != nil {
