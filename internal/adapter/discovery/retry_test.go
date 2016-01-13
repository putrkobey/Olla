package discovery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// failNTimes fails the first n calls with err, then returns a model.
func failNTimes(n int32, err error, calls *atomic.Int32) *discoveryFuncClient {
	return &discoveryFuncClient{discover: func(context.Context, *domain.Endpoint) ([]*domain.ModelInfo, error) {
		if calls.Add(1) <= n {
			return nil, err
		}
		return []*domain.ModelInfo{{Name: "m"}}, nil
	}}
}

func retryService(client ModelDiscoveryClient, ep *domain.Endpoint, attempts int, reg *mockModelRegistry) *ModelDiscoveryService {
	return NewModelDiscoveryService(client, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{ep}}, reg,
		DiscoveryConfig{Timeout: time.Second, ConcurrentWorkers: 1, RetryAttempts: attempts, RetryBackoff: time.Millisecond}, createTestLogger())
}

func TestDiscoveryRetry_RecoversFromTransientFailures(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	transient := NewDiscoveryError(ep.URLString, "ollama", "http_status", 503, 0, errors.New("unavailable"))

	var calls atomic.Int32
	reg := &mockModelRegistry{}
	svc := retryService(failNTimes(2, transient, &calls), ep, 3, reg)

	if err := svc.DiscoverEndpoint(context.Background(), ep); err != nil {
		t.Fatalf("expected recovery on the third attempt, got %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls=%d, want 3", calls.Load())
	}
	if reg.registerCallCount != 1 {
		t.Fatalf("models not registered after a recovered fetch")
	}
	if svc.getFailureCount(ep.URLString) != 0 {
		t.Fatal("recovered attempts must not count as failures")
	}
}

func TestDiscoveryRetry_ExhaustedCountsAsOneFailure(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	netErr := &NetworkError{URL: ep.URLString, Err: errors.New("connection refused")}

	var calls atomic.Int32
	svc := retryService(failNTimes(100, netErr, &calls), ep, 3, &mockModelRegistry{})

	if err := svc.DiscoverEndpoint(context.Background(), ep); err == nil {
		t.Fatal("expected failure once retries are spent")
	}
	if calls.Load() != 4 {
		t.Fatalf("calls=%d, want 1 attempt + 3 retries", calls.Load())
	}
	// four failed requests, one failed cycle: the five-failure cooldown must not
	// be reachable from a single cycle
	if got := svc.getFailureCount(ep.URLString); got != 1 {
		t.Fatalf("failure count=%d, want 1", got)
	}
	if svc.isEndpointDisabled(ep.URLString) {
		t.Fatal("one cycle must not trip the cooldown")
	}
}

func TestDiscoveryRetry_NonRetryableErrorsAreNotRetried(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	cases := map[string]error{
		"4xx":      NewDiscoveryError(ep.URLString, "ollama", "http_status", 404, 0, errors.New("not found")),
		"parse":    &ParseError{Format: "json", Err: errors.New("bad body")},
		"4xx wrap": NewDiscoveryError(ep.URLString, "ollama", "http_status", 401, 0, errors.New("unauthorised")),
		"profile":  &ProfileNotFoundError{ProfileType: "nope"},
		// the same oversized body comes back on every attempt
		"tls unknown authority":    &NetworkError{URL: ep.URLString, Err: x509.UnknownAuthorityError{}},
		"tls invalid cert":         &NetworkError{URL: ep.URLString, Err: x509.CertificateInvalidError{Reason: x509.Expired}},
		"tls verification wrapper": &NetworkError{URL: ep.URLString, Err: &tls.CertificateVerificationError{Err: x509.UnhandledCriticalExtension{}}},
		"tls hostname":             &NetworkError{URL: ep.URLString, Err: x509.HostnameError{Certificate: &x509.Certificate{}, Host: "backend"}},
		"oversized":                NewDiscoveryError(ep.URLString, "ollama", "read_response", 200, 0, fmt.Errorf("model response exceeds 10 bytes: %w", ErrResponseTooLarge)),
	}
	for name, failure := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			svc := retryService(failNTimes(100, failure, &calls), ep, 3, &mockModelRegistry{})
			if err := svc.DiscoverEndpoint(context.Background(), ep); err == nil {
				t.Fatal("expected failure")
			}
			if calls.Load() != 1 {
				t.Fatalf("calls=%d, want 1", calls.Load())
			}
		})
	}
}

func TestDiscoveryRetry_ZeroAttemptsMeansOneRequest(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	var calls atomic.Int32
	svc := retryService(failNTimes(100, &NetworkError{URL: "x", Err: errors.New("down")}, &calls), ep, 0, &mockModelRegistry{})
	_ = svc.DiscoverEndpoint(context.Background(), ep)
	if calls.Load() != 1 {
		t.Fatalf("calls=%d, want 1", calls.Load())
	}
}

func TestDiscoveryRetry_NegativeBackoffBehavesAsZero(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	var calls atomic.Int32
	svc := NewModelDiscoveryService(failNTimes(100, &NetworkError{URL: "x", Err: errors.New("down")}, &calls),
		&mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{ep}}, &mockModelRegistry{},
		DiscoveryConfig{Timeout: time.Second, ConcurrentWorkers: 1, RetryAttempts: 2, RetryBackoff: -time.Hour}, createTestLogger())
	if err := svc.DiscoverEndpoint(context.Background(), ep); err == nil {
		t.Fatal("expected failure once retries are spent")
	}
	if calls.Load() != 3 {
		t.Fatalf("calls=%d, want 1 attempt + 2 retries", calls.Load())
	}
}

func TestDiscoveryRetry_HonoursCancellationDuringBackoff(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	var calls atomic.Int32
	client := failNTimes(100, &NetworkError{URL: "x", Err: errors.New("down")}, &calls)
	svc := NewModelDiscoveryService(client, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{ep}}, &mockModelRegistry{},
		DiscoveryConfig{Timeout: 2 * time.Hour, ConcurrentWorkers: 1, RetryAttempts: 5, RetryBackoff: time.Hour}, createTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.DiscoverEndpoint(ctx, ep) }()

	deadline := time.After(5 * time.Second)
	for calls.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("first attempt never ran")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("retry backoff ignored cancellation")
	}
	if got := svc.getFailureCount(ep.URLString); got != 0 {
		t.Fatalf("cancelled cycle counted %d failures", got)
	}
}

func TestDiscoveryRetry_StopDuringBackoffIsNotAFailure(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	var calls atomic.Int32
	client := failNTimes(100, &NetworkError{URL: "x", Err: errors.New("down")}, &calls)
	svc := NewModelDiscoveryService(client, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{ep}}, &mockModelRegistry{},
		DiscoveryConfig{Interval: time.Hour, Timeout: 2 * time.Hour, ConcurrentWorkers: 1, RetryAttempts: 5, RetryBackoff: time.Hour}, createTestLogger())
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- svc.DiscoverEndpoint(context.Background(), ep) }()

	deadline := time.After(5 * time.Second)
	for calls.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("first attempt never ran")
		case <-time.After(time.Millisecond):
		}
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v, want a cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("retry backoff ignored Stop")
	}
	if got := svc.getFailureCount(ep.URLString); got != 0 {
		t.Fatalf("shutdown counted %d failures against the endpoint", got)
	}
}

// The synchronous pass at startup rides on this: however many retries are
// configured, a cycle must not outlast a single Timeout.
func TestDiscoveryRetry_CycleStaysWithinOneTimeout(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	transient := NewDiscoveryError(ep.URLString, "ollama", "http_status", 503, 0, errors.New("unavailable"))

	t.Run("fast failures stop retrying once the budget is spent", func(t *testing.T) {
		var calls atomic.Int32
		svc := NewModelDiscoveryService(failNTimes(1000, transient, &calls), &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{ep}}, &mockModelRegistry{},
			DiscoveryConfig{Timeout: 100 * time.Millisecond, ConcurrentWorkers: 1, RetryAttempts: 50, RetryBackoff: 40 * time.Millisecond}, createTestLogger())

		start := time.Now()
		if err := svc.DiscoverEndpoint(context.Background(), ep); err == nil {
			t.Fatal("expected failure")
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("cycle took %v with a 100ms timeout", elapsed)
		}
		// 40ms then 80ms backoff cannot both fit in 100ms
		if got := calls.Load(); got < 1 || got > 3 {
			t.Fatalf("calls=%d, want the budget to allow at most 3", got)
		}
		if got := svc.getFailureCount(ep.URLString); got != 1 {
			t.Fatalf("failure count=%d, want 1", got)
		}
	})

	t.Run("a hung backend is asked once", func(t *testing.T) {
		var calls atomic.Int32
		hung := &discoveryFuncClient{discover: func(ctx context.Context, _ *domain.Endpoint) ([]*domain.ModelInfo, error) {
			calls.Add(1)
			<-ctx.Done()
			return nil, &NetworkError{URL: "x", Err: ctx.Err()}
		}}
		svc := NewModelDiscoveryService(hung, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{ep}}, &mockModelRegistry{},
			DiscoveryConfig{Timeout: 100 * time.Millisecond, ConcurrentWorkers: 1, RetryAttempts: 3, RetryBackoff: time.Millisecond}, createTestLogger())

		start := time.Now()
		if err := svc.DiscoverEndpoint(context.Background(), ep); err == nil {
			t.Fatal("expected failure")
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("cycle took %v with a 100ms timeout", elapsed)
		}
		if calls.Load() != 1 {
			t.Fatalf("calls=%d, want 1: a timed-out request has no budget left to retry", calls.Load())
		}
	})
}

func TestDiscoveryRetry_SingleRetryAttempt(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	transient := NewDiscoveryError(ep.URLString, "ollama", "http_status", 503, 0, errors.New("unavailable"))

	t.Run("recovers on the one retry", func(t *testing.T) {
		var calls atomic.Int32
		svc := retryService(failNTimes(1, transient, &calls), ep, 1, &mockModelRegistry{})
		if err := svc.DiscoverEndpoint(context.Background(), ep); err != nil {
			t.Fatalf("expected recovery, got %v", err)
		}
		if calls.Load() != 2 {
			t.Fatalf("calls=%d, want 2", calls.Load())
		}
	})

	t.Run("stops after the one retry", func(t *testing.T) {
		var calls atomic.Int32
		svc := retryService(failNTimes(100, transient, &calls), ep, 1, &mockModelRegistry{})
		if err := svc.DiscoverEndpoint(context.Background(), ep); err == nil {
			t.Fatal("expected failure")
		}
		if calls.Load() != 2 {
			t.Fatalf("calls=%d, want 2", calls.Load())
		}
		if got := svc.getFailureCount(ep.URLString); got != 1 {
			t.Fatalf("failure count=%d, want 1", got)
		}
	})
}

// A wait that cannot fit in the cycle budget is skipped rather than slept, so
// the cycle neither extends Timeout nor sits idle until it expires.
func TestDiscoveryRetry_BackoffLargerThanRemainingTimeoutIsSkipped(t *testing.T) {
	ep := createMockEndpoint("http://backend", "backend")
	var calls atomic.Int32
	client := failNTimes(100, &NetworkError{URL: "x", Err: errors.New("down")}, &calls)
	svc := NewModelDiscoveryService(client, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{ep}}, &mockModelRegistry{},
		DiscoveryConfig{Timeout: 500 * time.Millisecond, ConcurrentWorkers: 1, RetryAttempts: 5, RetryBackoff: time.Hour}, createTestLogger())
