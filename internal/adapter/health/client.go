package health

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/putrkobey/Olla/internal/version"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/util"
)

const (
	DefaultMaxRetries = 2
	DefaultBaseDelay  = 100 * time.Millisecond
	MaxBackoffDelay   = 2 * time.Second
)

type HealthClient struct {
	client         HTTPClient
	circuitBreaker *CircuitBreaker
}

func NewHealthClient(client HTTPClient, circuitBreaker *CircuitBreaker) *HealthClient {
	return &HealthClient{
		client:         client,
		circuitBreaker: circuitBreaker,
	}
}

// Check performs a single health check against an endpoint with retry logic and panic recovery
func (hc *HealthClient) Check(ctx context.Context, endpoint *domain.Endpoint) (result domain.HealthCheckResult, err error) {
	// Panic recovery for critical path
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("health check panic recovered: %v", r)
			result = domain.HealthCheckResult{
				Status:     domain.StatusOffline,
				Error:      err,
				ErrorType:  domain.ErrorTypeHTTPError,
				Latency:    0,
				StatusCode: 0, // No HTTP response during panic
			}
		}
	}()

	healthCheckURL := endpoint.GetHealthCheckURLString()

	// Check circuit breaker first. attempt correlates this specific admitted
	// probe (half-open case) with the RecordSuccess/RecordFailure call below -
	// see CircuitBreaker.RecordSuccess for why that matters.
	circuitOpen, attempt := hc.circuitBreaker.IsOpen(healthCheckURL, endpoint.CheckTimeout)
	if circuitOpen {
		result = domain.HealthCheckResult{
			Status:     domain.StatusOffline,
			Error:      ErrCircuitBreakerOpen,
			ErrorType:  domain.ErrorTypeCircuitOpen,
			Latency:    0,
			StatusCode: 0, // No HTTP response when circuit breaker is open
		}
		return result, domain.NewHealthCheckError(endpoint, "circuit_breaker", 0, 0, ErrCircuitBreakerOpen)
	}

	// Perform check with retry logic
	var lastErr error
	overallStart := time.Now()
	maxRetries := DefaultMaxRetries
