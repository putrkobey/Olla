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

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := calculateBackoffDelay(attempt)

			// Use a separate context for the delay to avoid cancelling the main operation
			delayCtx, delayCancel := context.WithTimeout(context.Background(), delay)
			select {
			case <-delayCtx.Done():
				// Delay completed normally
			case <-ctx.Done():
				delayCancel()
				// Main context cancelled, stop retrying
				result.Latency = time.Since(overallStart)
				return result, domain.NewHealthCheckError(endpoint, "retry_cancelled", 0, result.Latency, ctx.Err())
			}
			delayCancel()
		}

		result, lastErr = hc.performSingleCheck(ctx, endpoint, healthCheckURL)

		// Check if we should retry
		if lastErr == nil || !shouldRetry(lastErr, result.ErrorType) {
			break
		}
	}

	// Record overall latency including retries
	result.Latency = time.Since(overallStart)

	// Record result in circuit breaker.
	// ConfigError and RateLimited are not service failures. The backend is up and
	// responding; counting them as failures would trip the CB on misconfigured
	// credentials or a throttled endpoint, hiding the real cause from the operator.
	if lastErr != nil {
		hc.circuitBreaker.RecordFailure(healthCheckURL, attempt)
	} else {
		switch result.Status {
		case domain.StatusConfigError, domain.StatusRateLimited:
			// Do not trip the circuit breaker; this is an operator or rate error, not downtime.
		case domain.StatusHealthy, domain.StatusBusy:
			hc.circuitBreaker.RecordSuccess(healthCheckURL, attempt)
		default:
			hc.circuitBreaker.RecordFailure(healthCheckURL, attempt)
		}
	}

	if lastErr != nil {
		return result, domain.NewHealthCheckError(endpoint, "health_check", result.StatusCode, result.Latency, lastErr)
	}

	return result, nil
}

func (hc *HealthClient) performSingleCheck(ctx context.Context, endpoint *domain.Endpoint, healthCheckURL string) (domain.HealthCheckResult, error) {
	start := time.Now()
	result := domain.HealthCheckResult{
		Status: domain.StatusUnknown,
	}

	checkCtx, cancel := context.WithTimeout(ctx, endpoint.CheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, healthCheckURL, http.NoBody)
	if err != nil {
		result.Latency = time.Since(start)
		result.Error = err
		result.ErrorType = classifyError(err)
		result.Status = determineStatus(0, result.Latency, err, result.ErrorType)
		result.StatusCode = 0 // No HTTP response received
		return result, err
	}

