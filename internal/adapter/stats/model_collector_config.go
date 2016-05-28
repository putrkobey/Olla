package stats

import "time"

// ModelCollectorConfig provides configuration options for the ModelCollector
type ModelCollectorConfig struct {

	// PercentileTrackerType specifies which percentile tracker to use ("reservoir", "simple", default: "reservoir")
	PercentileTrackerType string

	// MaxTrackedModels is the maximum number of models to track (default: 50)
	MaxTrackedModels int

	// ModelStatsTTL is how long to keep model statistics (default: 6h)
	ModelStatsTTL time.Duration

	// ModelCleanupInterval is how often to clean up old data (default: 30m)
	ModelCleanupInterval time.Duration

	// MaxUniqueClientsPerModel is the maximum number of unique clients to track per model (default: 100)
	MaxUniqueClientsPerModel int

	// PercentileSampleSize is the size of the reservoir for percentile calculation (default: 100)
	PercentileSampleSize int

	// ClientIPRetentionTime is how long to keep client IP records (default: 1h)
	ClientIPRetentionTime time.Duration

	// EnableDetailedStats enables detailed model-endpoint statistics tracking (default: false)
	EnableDetailedStats bool
}

// DefaultModelCollectorConfig returns the default configuration
func DefaultModelCollectorConfig() *ModelCollectorConfig {
	return &ModelCollectorConfig{
