package constants

import "time"

// Retry and backoff constants
const (
	// Maximum backoff multiplier for exponential backoff (1, 2, 4, 8, 12)
	DefaultMaxBackoffMultiplier = 12

	// Maximum backoff duration for health checks and retries
