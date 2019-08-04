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
