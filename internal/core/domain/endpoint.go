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
