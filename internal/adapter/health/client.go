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

	req = injectDefaultHeaders(req)
	injectEndpointAuth(req, endpoint)
	resp, err := hc.client.Do(req)
	result.Latency = time.Since(start)

	if err != nil {
		result.Error = err
		result.ErrorType = classifyError(err)
		result.Status = determineStatus(0, result.Latency, err, result.ErrorType)
		result.StatusCode = 0 // No HTTP response received
		return result, err
	}

	// SHERPA-64: Resource leak post HTTP request fails after extended query
	//  14-Aug-2024 [ML] this was causing connection reuse issues across multiple health checks
	// 		 			 but interestingly, repro was mostly seen with LMStudio endpoints
	defer func() {
		// Ensure response body is fully read and closed to enable connection reuse
		if resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}()

	// Capture HTTP status code and determine endpoint status
	result.StatusCode = resp.StatusCode
	result.Status = determineStatus(resp.StatusCode, result.Latency, nil, domain.ErrorTypeNone)

	if resp.StatusCode == http.StatusTooManyRequests {
		result.RateLimitedUntil = parseRetryAfter(resp.Header.Get("Retry-After"), endpoint.Name)
	}

	return result, nil
}

// parseRetryAfter interprets the Retry-After header value per RFC 9110.
// It accepts delay-seconds and HTTP-date formats. Falls back to DefaultRateLimitBackoff
// if the value is missing or malformed.
func parseRetryAfter(header, endpointName string) time.Time {
	now := time.Now()

	if header == "" {
		slog.Info("no Retry-After header on 429, using default backoff",
			"endpoint", endpointName,
			"default", DefaultRateLimitBackoff)
		return now.Add(DefaultRateLimitBackoff)
	}

	// Try delay-seconds first (most common for API services).
	if secs, err := strconv.ParseInt(header, 10, 64); err == nil {
		return now.Add(time.Duration(secs) * time.Second)
	}

	// Try HTTP-date format (RFC 1123 / RFC 850 / ANSI C asctime).
	for _, layout := range []string{http.TimeFormat, "Monday, 02-Jan-06 15:04:05 MST", "Mon Jan _2 15:04:05 2006"} {
		if t, err := time.Parse(layout, header); err == nil {
			return t
		}
	}

	slog.Info("malformed Retry-After header on 429, using default backoff",
		"endpoint", endpointName,
		"header", header,
		"default", DefaultRateLimitBackoff)
	return now.Add(DefaultRateLimitBackoff)
}

func injectDefaultHeaders(req *http.Request) *http.Request {
	req.Header.Set("User-Agent", fmt.Sprintf("%s-HealthChecker/%s", version.ShortName, version.Version))
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cache-Control", "no-cache")
	return req
}

