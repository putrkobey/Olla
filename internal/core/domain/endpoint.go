package domain

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

const (
	StatusStringHealthy     = "healthy"
	StatusStringBusy        = "busy"
	StatusStringOffline     = "offline"
	StatusStringWarming     = "warming"
	StatusStringUnhealthy   = "unhealthy"
	StatusStringUnknown     = "unknown"
	StatusStringConfigError = "config_error"
	StatusStringRateLimited = "rate_limited"
)

type Endpoint struct {
	LastChecked   time.Time
	NextCheckTime time.Time
	// RateLimitedUntil is set when a health probe receives 429. The scheduler skips
	// probing this endpoint until the time passes. Never serialised.
	RateLimitedUntil time.Time `json:"-"`
	URL              *url.URL
	HealthCheckURL   *url.URL
	ModelUrl         *url.URL
	ModelFilter      *FilterConfig
	// Headers holds verbatim outbound headers copied from endpoint config at load time.
	Headers               map[string]string `json:"-"`
	Name                  string
	Type                  string `json:"type,omitempty"`
	Status                EndpointStatus
	URLString             string
	HealthCheckPathString string
	HealthCheckURLString  string
	ModelURLString        string
	// AuthHeaderName is the resolved header name for outbound auth (e.g. "Authorization", "X-Api-Key").
	// Precomputed at load time so the hot path pays no allocation cost.
	// Never serialised: exposing the header name through status endpoints would reveal the
	// auth scheme in use, making credential-guessing attacks cheaper.
	AuthHeaderName string `json:"-"`
	// AuthHeaderValue is the fully composed header value (e.g. "Bearer tok", "Basic base64(...)").
	// Never serialised; leaking credentials through logs or status endpoints would be a security issue.
	AuthHeaderValue     string `json:"-"`
	LastLatency         time.Duration
	CheckInterval       time.Duration
	CheckTimeout        time.Duration
	Priority            int
	ConsecutiveFailures int
	BackoffMultiplier   int
	PreservePath        bool
}

func (e *Endpoint) GetURLString() string {
	return e.URLString
}

func (e *Endpoint) GetHealthCheckURLString() string {
	return e.HealthCheckURLString
}

