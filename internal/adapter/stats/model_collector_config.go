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
