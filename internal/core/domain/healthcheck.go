package domain

import (
	"context"
	"time"
)

type HealthCheckResult struct {
	// RateLimitedUntil is populated when the probe received a 429 with a Retry-After
	// header. The scheduler uses this to skip probing until the window has elapsed.
	RateLimitedUntil time.Time
	Error            error
	Status           EndpointStatus
