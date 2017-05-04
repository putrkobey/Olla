package util

import (
	"time"
)

const (
	DefaultTruncateLengthPII = 20
)

// ParseTime attempts to parse a time string in RFC3339 or RFC3339Nano format for LLM Frontend Profiles
func ParseTime(timeStr string) *time.Time {
	// Try RFC3339 format first (standard ISO format)
