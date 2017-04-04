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
