package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// rateLimitBackend returns 429 with an optional Retry-After header.
func rateLimitBackend(t *testing.T, retryAfterHeader string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfterHeader != "" {
			w.Header().Set("Retry-After", retryAfterHeader)
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func rlEndpoint(t *testing.T, rawURL string) *domain.Endpoint {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return &domain.Endpoint{
		Name:                 "rl-test",
		URL:                  u,
		HealthCheckURL:       u,
		URLString:            u.String(),
		HealthCheckURLString: u.String(),
		CheckTimeout:         2 * time.Second,
	}
}

// TestRateLimit_RetryAfterSeconds verifies that a numeric Retry-After is parsed
// into a RateLimitedUntil approximately 60s in the future.
func TestRateLimit_RetryAfterSeconds(t *testing.T) {
	t.Parallel()

	srv := rateLimitBackend(t, "60")
	ep := rlEndpoint(t, srv.URL)

	hc := NewHealthClient(http.DefaultClient, NewCircuitBreaker())
	before := time.Now()
	result, err := hc.Check(context.Background(), ep)
	after := time.Now()

	require.NoError(t, err)
	assert.Equal(t, domain.StatusRateLimited, result.Status)
	assert.False(t, result.RateLimitedUntil.IsZero(), "RateLimitedUntil must be set")

	// Allow a generous window for test execution jitter.
	lo := before.Add(59 * time.Second)
	hi := after.Add(61 * time.Second)
	assert.True(t, result.RateLimitedUntil.After(lo) && result.RateLimitedUntil.Before(hi),
		"RateLimitedUntil (%v) should be ~60s from probe time [%v, %v]",
		result.RateLimitedUntil, lo, hi)
}

// TestRateLimit_RetryAfterHTTPDate verifies that an HTTP-date Retry-After is parsed.
func TestRateLimit_RetryAfterHTTPDate(t *testing.T) {
	t.Parallel()

	future := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Second)
	httpDate := future.Format(http.TimeFormat)

	srv := rateLimitBackend(t, httpDate)
	ep := rlEndpoint(t, srv.URL)

	hc := NewHealthClient(http.DefaultClient, NewCircuitBreaker())
	result, err := hc.Check(context.Background(), ep)

	require.NoError(t, err)
	assert.Equal(t, domain.StatusRateLimited, result.Status)
	assert.False(t, result.RateLimitedUntil.IsZero())
	// Should be within a second of our expected time.
