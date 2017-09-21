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
	data := mc.getOrInitModel(model, now)
	data.totalRequests.Inc()
	data.totalBytes.Add(bytes)
	data.totalLatency.Add(latencyMs)
	atomic.StoreInt64(&data.lastRequested, now)

	if status == StatusSuccess {
		data.successfulRequests.Inc()
		mc.recordLatency(data, latencyMs)
	} else {
		data.failedRequests.Inc()
	}

	// Update model-endpoint stats only if detailed stats are enabled
	if mc.config.EnableDetailedStats && endpoint != nil {
		mc.updateModelEndpointStats(model, endpoint, status, latencyMs, now)
	}

	mc.tryCleanup(now)
}

func (mc *ModelCollector) RecordModelError(model string, endpoint *domain.Endpoint, errorType string) {
	if !mc.config.EnableDetailedStats || model == "" || endpoint == nil {
		return
	}

	endpointMap, _ := mc.modelEndpoints.LoadOrStore(model, xsync.NewMap[string, *modelEndpointData]())
	if data, ok := endpointMap.Load(endpoint.Name); ok {
		atomic.AddInt32(&data.consecutiveErrors, 1)
	}
}

func (mc *ModelCollector) GetModelStats() map[string]ports.ModelStats {
	result := make(map[string]ports.ModelStats)

	mc.models.Range(func(name string, data *modelData) bool {
		stats := mc.calculateModelStats(data)
		result[name] = stats
		return true
	})

	return result
}

func (mc *ModelCollector) GetModelEndpointStats() map[string]map[string]ports.EndpointModelStats {
	result := make(map[string]map[string]ports.EndpointModelStats)

	// Return empty map if detailed stats are disabled
	if !mc.config.EnableDetailedStats {
		return result
	}

	mc.modelEndpoints.Range(func(modelName string, endpointMap *xsync.Map[string, *modelEndpointData]) bool {
		endpointStats := make(map[string]ports.EndpointModelStats)

		endpointMap.Range(func(endpointName string, data *modelEndpointData) bool {
			stats := mc.calculateEndpointModelStats(data)
			endpointStats[endpointName] = stats
			return true
		})

		result[modelName] = endpointStats
		return true
	})

	return result
}

func (mc *ModelCollector) getOrInitModel(model string, now int64) *modelData {
	data, _ := mc.models.LoadOrCompute(model, func() (*modelData, bool) {
		var tracker PercentileTracker

		// Create appropriate percentile tracker based on config
		switch mc.config.PercentileTrackerType {
		case "simple":
			tracker = NewSimpleStatsTracker()
		default: // "reservoir"
			tracker = NewReservoirSampler(mc.config.PercentileSampleSize)
		}

