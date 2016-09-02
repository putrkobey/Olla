package stats

/*
				Olla Stats Collector - Centralised Stats Collection
	Collector centralises all the stats we track across Olla - requests, connections,
	security stuff, etc. Instead of each component doing its own thing, everything
	reports here so we can actually see what's happening system-wide.

	Thread-safe for high concurrency since this gets hit on every request & multiple.
	times. We also clean up old endpoint data automatically so we don't leak memory.

	NOTE: 	Cleanup values defined are conservative to avoid memory retention over
		  	long running processes. Most users will have 10-20 endpoints, so we keep
			the tracked endpoints to a maximum of 50.

	GOALS:
	- Keep it simple and efficient (reduce allocation overhead)
	- Track all relevant stats in one place
	- Provide easy access to stats for monitoring and debugging
	- Automatically clean up old data to prevent long memory retention
*/

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/putrkobey/Olla/internal/core/domain"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	StatusSuccess = "success"
	StatusFailure = "failure"

	// NOTE: These are not too high to avoid memory retention for long periods
	// Most folks would have 10-20 endpoints looking at Sherpa usage stats
	MaxTrackedEndpoints = 50
	EndpointTTL         = 1 * time.Hour
	CleanupInterval     = 5 * time.Minute

	// MaxUniqueRateLimitedIPs caps the in-memory set of rate-limited client IPs.
	// Under an active flood attack the map would otherwise grow without bound.
	// At cap, new entries are refused until the periodic age-eviction frees space.
	MaxUniqueRateLimitedIPs = 10_000

	// ipCleanupInterval is how often the age-based eviction scan runs. Keeping
	// it off the per-violation hot path avoids an O(N) scan under high load.
	ipCleanupInterval = 5 * time.Minute
)

type Collector struct {
	logger logger.StyledLogger

	uniqueRateLimitedIPs map[string]int64

	endpoints *xsync.Map[string, *endpointData]

	// Model statistics tracking
	modelCollector *ModelCollector

	// Translator statistics tracking
	translatorCollector *TranslatorCollector

	// Using xsync.Counter for better performance under high contention
	totalRequests      *xsync.Counter
	successfulRequests *xsync.Counter
	failedRequests     *xsync.Counter
	totalLatency       *xsync.Counter

	rateLimitViolations *xsync.Counter
	sizeLimitViolations *xsync.Counter

	// Global latency bounds across all endpoints, CAS-updated the same way as
	// endpointData's per-endpoint bounds (see updateLatencyBounds). minLatency
	// starts at -1 as an "unset" sentinel since 0 is a valid observed latency.
	minLatency int64
	maxLatency int64

	lastCleanup   int64
	lastIPCleanup int64 // atomic: tracks when the age-eviction scan last ran
	securityMu    sync.RWMutex

	cleanupMu sync.Mutex
}

type endpointData struct {
	totalRequests      *xsync.Counter
	successfulRequests *xsync.Counter
	failedRequests     *xsync.Counter
	totalBytes         *xsync.Counter
	totalLatency       *xsync.Counter
	name               string
	url                string
	activeConnections  int64 // Still using atomic for decrement operations
	minLatency         int64 // Keep atomic for CAS operations
	maxLatency         int64 // Keep atomic for CAS operations
	lastUsed           int64 // Keep atomic for timestamp
}

func NewCollector(logger logger.StyledLogger) *Collector {
	return NewCollectorWithConfig(logger, DefaultModelCollectorConfig())
}

// NewCollectorWithConfig creates a new Collector with custom ModelCollector configuration
func NewCollectorWithConfig(logger logger.StyledLogger, modelConfig *ModelCollectorConfig) *Collector {
	if modelConfig == nil {
		modelConfig = DefaultModelCollectorConfig()
	}

	return &Collector{
		uniqueRateLimitedIPs: make(map[string]int64),
		logger:               logger,
		endpoints:            xsync.NewMap[string, *endpointData](),
		lastCleanup:          time.Now().UnixNano(),
		minLatency:           -1,
		modelCollector:       NewModelCollectorWithConfig(modelConfig),
		translatorCollector:  NewTranslatorCollector(),
		totalRequests:        xsync.NewCounter(),
		successfulRequests:   xsync.NewCounter(),
		failedRequests:       xsync.NewCounter(),
		totalLatency:         xsync.NewCounter(),
		rateLimitViolations:  xsync.NewCounter(),
		sizeLimitViolations:  xsync.NewCounter(),
	}
}

func (c *Collector) RecordRequest(endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
	now := time.Now().UnixNano()
	latencyMs := latency.Milliseconds()

	c.totalRequests.Inc()

	if status == StatusSuccess {
		c.successfulRequests.Inc()
		// Update total latency only for successful requests
		// realised in TestCollector_RecordRequest
		c.totalLatency.Add(latencyMs)
		c.updateGlobalLatencyBounds(latencyMs)
	} else {
		c.failedRequests.Inc()
	}

	// Only update endpoint-specific stats if endpoint is known
	if endpoint != nil {
		c.updateEndpointStats(endpoint, status, latencyMs, bytes, now)
	}
	c.tryCleanup(now)
}

func (c *Collector) RecordConnection(endpoint *domain.Endpoint, delta int) {
	now := time.Now().UnixNano()
	data := c.getOrInitEndpoint(endpoint, now)

	if delta > 0 {
		atomic.AddInt64(&data.activeConnections, int64(delta))
	} else if delta < 0 {
		for {
			current := atomic.LoadInt64(&data.activeConnections)
			newVal := current + int64(delta)
			if newVal < 0 {
				newVal = 0
			}
			if atomic.CompareAndSwapInt64(&data.activeConnections, current, newVal) {
				break
			}
		}
	}
}

func (c *Collector) RecordDiscovery(endpoint *domain.Endpoint, success bool, latency time.Duration) {
	status := StatusFailure
	if success {
		status = StatusSuccess
	}

	c.logger.Debug("Discovery operation recorded",
		"endpoint", endpoint,
		"status", status,
		"latency_ms", latency.Milliseconds())
}

func (c *Collector) GetProxyStats() ports.ProxyStats {
	total := c.totalRequests.Value()
	successful := c.successfulRequests.Value()
	failed := c.failedRequests.Value()
	totalLatency := c.totalLatency.Value()

	var avgLatency int64
	if successful > 0 {
		avgLatency = totalLatency / successful
	}

	minLatency := atomic.LoadInt64(&c.minLatency)
	if minLatency == -1 {
		minLatency = 0
	}

	return ports.ProxyStats{
		TotalRequests:      total,
		SuccessfulRequests: successful,
		FailedRequests:     failed,
		AverageLatency:     avgLatency,
		MinLatency:         minLatency,
		MaxLatency:         atomic.LoadInt64(&c.maxLatency),
	}
}

// updateGlobalLatencyBounds is the global-across-all-endpoints equivalent of
// updateLatencyBounds, feeding GetProxyStats' MinLatency/MaxLatency.
func (c *Collector) updateGlobalLatencyBounds(latencyMs int64) {
	for {
		minLatency := atomic.LoadInt64(&c.minLatency)
		if minLatency == -1 || latencyMs < minLatency {
			if atomic.CompareAndSwapInt64(&c.minLatency, minLatency, latencyMs) {
				break
			}
		} else {
			break
		}
	}
	for {
		maxLatency := atomic.LoadInt64(&c.maxLatency)
		if latencyMs > maxLatency {
			if atomic.CompareAndSwapInt64(&c.maxLatency, maxLatency, latencyMs) {
				break
			}
		} else {
			break
		}
	}
}

func (c *Collector) GetEndpointStats() map[string]ports.EndpointStats {
	stats := make(map[string]ports.EndpointStats)

	c.endpoints.Range(func(url string, data *endpointData) bool {
		total := data.totalRequests.Value()
		successful := data.successfulRequests.Value()
		failed := data.failedRequests.Value()
		totalLatency := data.totalLatency.Value()
		avgLatency := int64(0)
		if successful > 0 {
			avgLatency = totalLatency / successful
		}

		successRate := 0.0
		if total > 0 {
