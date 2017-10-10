package balancer

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestNewRoundRobinSelector(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())

	if selector == nil {
		t.Fatal("NewRoundRobinSelector returned nil")
	}

	if selector.statsCollector.GetConnectionStats() == nil {
		t.Error("Connections map not initialised")
	}

	if selector.Name() != DefaultBalancerRoundRobin {
		t.Errorf("Expected name '%s', got %q", DefaultBalancerRoundRobin, selector.Name())
	}

	// Counter should start at 0
	if selector.counter != 0 {
		t.Errorf("Expected counter to start at 0, got %d", selector.counter)
	}
}

func TestRoundRobinSelector_Select_NoEndpoints(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoint, err := selector.Select(ctx, []*domain.Endpoint{})
	if err == nil {
		t.Error("Expected error for empty endpoints")
	}
	if endpoint != nil {
		t.Error("Expected nil endpoint for empty slice")
	}
}

func TestRoundRobinSelector_Select_NoRoutableEndpoints(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createRoundRobinEndpoint("offline", 11434, domain.StatusOffline),
		createRoundRobinEndpoint("unhealthy", 11435, domain.StatusUnhealthy),
		createRoundRobinEndpoint("unknown", 11436, domain.StatusUnknown),
	}

	endpoint, err := selector.Select(ctx, endpoints)
	if err == nil {
		t.Error("Expected error for no routable endpoints")
	}
	if endpoint != nil {
		t.Error("Expected nil endpoint for no routable endpoints")
	}
}

func TestRoundRobinSelector_Select_SingleEndpoint(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createRoundRobinEndpoint("single", 11434, domain.StatusHealthy),
	}

	// Should always return the same endpoint
	for range 5 {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		if endpoint == nil {
			t.Fatal("Expected endpoint, got nil")
		}
		if endpoint.Name != "single" {
			t.Errorf("Expected 'single', got %s", endpoint.Name)
		}
	}
}

func TestRoundRobinSelector_Select_RoundRobinDistribution(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createRoundRobinEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createRoundRobinEndpoint("endpoint-2", 11435, domain.StatusHealthy),
		createRoundRobinEndpoint("endpoint-3", 11436, domain.StatusHealthy),
	}

	// Test sequential round-robin behaviour - starts from index 0
	expectedOrder := []string{"endpoint-1", "endpoint-2", "endpoint-3", "endpoint-1", "endpoint-2", "endpoint-3"}

	for i, expected := range expectedOrder {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
