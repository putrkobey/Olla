package ports

import (
	"context"
	"time"
)

type SecurityRequest struct {
	Headers       map[string][]string
	ClientID      string
	Endpoint      string
	Method        string
	BodySize      int64
	HeaderSize    int64
	IsHealthCheck bool
}

type SecurityResult struct {
	ResetTime  time.Time
	Reason     string
	RetryAfter int
	RateLimit  int
	Remaining  int
	Allowed    bool
}

type SecurityViolation struct {
	Timestamp     time.Time
	ClientID      string
	ViolationType string
	Endpoint      string
	Size          int64
}

type SecurityMetrics struct {
	RateLimitViolations  int64
	SizeLimitViolations  int64
	UniqueRateLimitedIPs int
}

type SecurityValidator interface {
	Validate(ctx context.Context, req SecurityRequest) (SecurityResult, error)
	Name() string
}

type SecurityChain struct {
	validators []SecurityValidator
}

