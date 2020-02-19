package stats

import (
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// TranslatorCollector tracks translator-specific statistics
type TranslatorCollector struct {
	translators *xsync.Map[string, *translatorData]
}

// translatorData holds all metrics for a single translator
type translatorData struct {
	// Total request counts
	totalRequests      *xsync.Counter
	successfulRequests *xsync.Counter
	failedRequests     *xsync.Counter

	// Mode breakdown
	passthroughRequests *xsync.Counter
	translationRequests *xsync.Counter

	// Streaming breakdown
	streamingRequests    *xsync.Counter
	nonStreamingRequests *xsync.Counter

	// Fallback reasons
	fallbackNoCompatibleEndpoints               *xsync.Counter
	fallbackTranslatorDoesNotSupportPassthrough *xsync.Counter
	fallbackCannotPassthrough                   *xsync.Counter

	// Performance metrics
	totalLatency *xsync.Counter
