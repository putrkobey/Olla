package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

type discoveryFuncClient struct {
	mockDiscoveryClient
	discover func(context.Context, *domain.Endpoint) ([]*domain.ModelInfo, error)
}

func (c *discoveryFuncClient) DiscoverModels(ctx context.Context, ep *domain.Endpoint) ([]*domain.ModelInfo, error) {
	return c.discover(ctx, ep)
}

func TestDiscoveryFailureDoesNotCancelOtherEndpoints(t *testing.T) {
	failed := createMockEndpoint("http://failed", "failed")
	healthy := createMockEndpoint("http://healthy", "healthy")
	failure := errors.New("backend failure")
	client := &discoveryFuncClient{discover: func(ctx context.Context, ep *domain.Endpoint) ([]*domain.ModelInfo, error) {
		if ep == failed {
			return nil, failure
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return []*domain.ModelInfo{{Name: "healthy-model"}}, nil
	}}
	registry := &mockModelRegistry{}
	service := NewModelDiscoveryService(client, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{failed, healthy}}, registry, DiscoveryConfig{Timeout: time.Second, ConcurrentWorkers: 1}, createTestLogger())
	err := service.DiscoverAll(context.Background())
	if !errors.Is(err, failure) {
		t.Fatalf("expected original failure, got %v", err)
	}
	if registry.registerCallCount != 1 || len(registry.registeredModels) != 1 || registry.registeredModels[0].Name != "healthy-model" {
		t.Fatalf("healthy endpoint was not registered: %+v", registry)
	}
	if service.getFailureCount(healthy.URLString) != 0 {
		t.Fatal("healthy endpoint penalised by sibling failure")
	}
}

func TestDiscoveryCancellationAndRequestTimeout(t *testing.T) {
	for _, parentCancelled := range []bool{true, false} {
		t.Run(fmt.Sprintf("parent_cancelled=%v", parentCancelled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			endpoint := createMockEndpoint("http://backend", "backend")
			client := &discoveryFuncClient{discover: func(ctx context.Context, _ *domain.Endpoint) ([]*domain.ModelInfo, error) {
				if parentCancelled {
					cancel()
				}
				<-ctx.Done()
				return nil, fmt.Errorf("request failed: %w", ctx.Err())
			}}
			service := NewModelDiscoveryService(client, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{endpoint}}, &mockModelRegistry{}, DiscoveryConfig{Timeout: time.Millisecond, ConcurrentWorkers: 1}, createTestLogger())
			err := service.DiscoverAll(ctx)
			expected := context.DeadlineExceeded
			failures := 1
			if parentCancelled {
				expected = context.Canceled
				failures = 0
			}
			if !errors.Is(err, expected) {
				t.Fatalf("expected %v, got %v", expected, err)
			}
			if got := service.getFailureCount(endpoint.URLString); got != failures {
				t.Fatalf("failures=%d, want %d", got, failures)
			}
		})
	}
}

func TestDiscoveryCooldownRecovery(t *testing.T) {
	endpoint := createMockEndpoint("http://backend", "backend")
	calls := 0
	broken := true
	client := &discoveryFuncClient{discover: func(context.Context, *domain.Endpoint) ([]*domain.ModelInfo, error) {
		calls++
		if broken {
			return nil, &ParseError{Format: "json", Err: errors.New("invalid response")}
		}
		return []*domain.ModelInfo{{Name: "recovered-model"}}, nil
	}}
	registry := &mockModelRegistry{}
	service := NewModelDiscoveryService(client, &mockEndpointRepository{healthyEndpoints: []*domain.Endpoint{endpoint}}, registry, DiscoveryConfig{Timeout: time.Second, ConcurrentWorkers: 1}, createTestLogger())
	for i := 1; i <= MaxConsecutiveFailures; i++ {
		if err := service.DiscoverAll(context.Background()); err == nil {
			t.Fatal("expected failure")
		}
		if i < MaxConsecutiveFailures && service.GetMetrics().ErrorsByEndpoint["_disabled_endpoints"] != 0 {
			t.Fatal("ordinary failure reported as disabled")
		}
	}
	if got := service.GetMetrics().ErrorsByEndpoint["_disabled_endpoints"]; got != 1 {
		t.Fatalf("disabled=%d", got)
	}
	if err := service.DiscoverAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != MaxConsecutiveFailures {
		t.Fatal("endpoint retried during cooldown")
	}
	expireCooldown := func() {
		service.mu.Lock()
		service.retryAfter[endpoint.URLString] = time.Now().Add(-time.Second)
		service.mu.Unlock()
	}
	expireCooldown()
	if err := service.DiscoverAll(context.Background()); err == nil {
		t.Fatal("expected failed recovery probe")
	}
	if !service.isEndpointDisabled(endpoint.URLString) {
		t.Fatal("failed probe did not restart cooldown")
	}
	expireCooldown()
	broken = false
	if err := service.DiscoverAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != MaxConsecutiveFailures+2 || registry.registerCallCount != 1 || service.getFailureCount(endpoint.URLString) != 0 {
		t.Fatal("successful recovery did not register models and reset failures")
	}
	if got := service.GetMetrics().ErrorsByEndpoint["_disabled_endpoints"]; got != 0 {
		t.Fatalf("recovered endpoint reported disabled: %d", got)
	}
}

func TestDiscoveryCooldownDuration(t *testing.T) {
	for _, interval := range []time.Duration{time.Second, 20 * time.Minute} {
		service := NewModelDiscoveryService(nil, nil, nil, DiscoveryConfig{Interval: interval}, createTestLogger())
		before := time.Now()
		service.disableEndpoint("backend")
		if service.retryAfter["backend"].Before(before.Add(max(DefaultDiscoveryCooldown, interval))) {
