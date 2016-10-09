package stats

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// ModelCollector tracks model-specific statistics
type ModelCollector struct {
	models         *xsync.Map[string, *modelData]
	modelEndpoints *xsync.Map[string, *xsync.Map[string, *modelEndpointData]]

	config *ModelCollectorConfig

	lastCleanup int64
	cleanupMu   sync.Mutex
}

type modelData struct {

	// Percentile tracking
	percentileTracker  PercentileTracker
	uniqueClients      *xsync.Map[string, int64] // IP -> last seen timestamp
	totalRequests      *xsync.Counter
	successfulRequests *xsync.Counter
	failedRequests     *xsync.Counter
	totalBytes         *xsync.Counter
	totalLatency       *xsync.Counter

	// Routing effectiveness
	routingHits      *xsync.Counter
	routingMisses    *xsync.Counter
	routingFallbacks *xsync.Counter
	name             string

	lastRequested int64 // Keep atomic for timestamp

	latencyMu sync.Mutex // Used for thread-safe percentile tracker access
}

type modelEndpointData struct {
	requestCount      *xsync.Counter
	successCount      *xsync.Counter
	totalLatency      *xsync.Counter
	endpointName      string
	modelName         string
	lastUsed          int64 // Keep atomic for timestamp
	consecutiveErrors int32 // Keep atomic for CAS operations
}

// NewModelCollectorWithConfig creates a new ModelCollector with custom configuration
func NewModelCollectorWithConfig(config *ModelCollectorConfig) *ModelCollector {
	if config == nil {
		config = DefaultModelCollectorConfig()
	}
	config.Validate()

	return &ModelCollector{
		models:         xsync.NewMap[string, *modelData](),
		modelEndpoints: xsync.NewMap[string, *xsync.Map[string, *modelEndpointData]](),
		lastCleanup:    time.Now().UnixNano(),
		config:         config,
	}
}

func (mc *ModelCollector) RecordModelRequest(model string, endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
	if model == "" {
		return
	}

	now := time.Now().UnixNano()
	latencyMs := latency.Milliseconds()

	// Update model stats
