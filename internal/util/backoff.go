package util

import (
	"math"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// CalculateExponentialBackoff computes exponential backoff with optional jitter.
// Formula: baseDelay * 2^(attempt-1), capped at maxDelay
func CalculateExponentialBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration, jitterPercent float64) time.Duration {
	if attempt <= 0 {
		return 0
