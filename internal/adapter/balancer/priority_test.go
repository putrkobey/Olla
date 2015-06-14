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

func TestNewPrioritySelector(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())

	if selector == nil {
		t.Fatal("NewPrioritySelector returned nil")
	}

	if selector.statsCollector.GetConnectionStats() == nil {
		t.Error("Connections map not initialised")
	}

	if selector.Name() != DefaultBalancerPriority {
		t.Errorf("Expected name '%s', got %q", DefaultBalancerPriority, selector.Name())
	}
}

func TestPrioritySelector_Select_NoEndpoints(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoint, err := selector.Select(ctx, []*domain.Endpoint{})
	if err == nil {
		t.Error("Expected error for empty endpoints")
	}
	if endpoint != nil {
		t.Error("Expected nil endpoint for empty slice")
	}
}

func TestPrioritySelector_Select_NoRoutableEndpoints(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createPriorityEndpoint("offline", 11434, domain.StatusOffline, 100),
		createPriorityEndpoint("unhealthy", 11435, domain.StatusUnhealthy, 200),
		createPriorityEndpoint("unknown", 11436, domain.StatusUnknown, 300),
	}

	endpoint, err := selector.Select(ctx, endpoints)
	if err == nil {
		t.Error("Expected error for no routable endpoints")
	}
	if endpoint != nil {
		t.Error("Expected nil endpoint for no routable endpoints")
	}
}

func TestPrioritySelector_Select_SingleEndpoint(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createPriorityEndpoint("single", 11434, domain.StatusHealthy, 100),
	}

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

func TestPrioritySelector_Select_HighestPriority(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createPriorityEndpoint("low", 11434, domain.StatusHealthy, 100),
		createPriorityEndpoint("high", 11435, domain.StatusHealthy, 300),
		createPriorityEndpoint("medium", 11436, domain.StatusHealthy, 200),
	}

	// Should always select hightest priority
	for range 10 {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		if endpoint.Name != "high" {
			t.Errorf("Expected highest priority 'high', got %s", endpoint.Name)
		}
	}
}

func TestPrioritySelector_Select_SamePriorityWeightedSelection(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	// All endpoints have same priority but different statuses
	endpoints := []*domain.Endpoint{
		createPriorityEndpoint("healthy", 11434, domain.StatusHealthy, 100),
		createPriorityEndpoint("busy", 11435, domain.StatusBusy, 100),
		createPriorityEndpoint("warming", 11436, domain.StatusWarming, 100),
	}

	selections := make(map[string]int)
	totalSelections := 1000

	// Run many selections to test weighted distribution
	for range totalSelections {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		selections[endpoint.Name]++
	}

	// Healthy should be selected most (weight 1.0)
	// Busy should be selected less (weight 0.3)
	// Warming should be selected least (weight 0.1)
	if selections["healthy"] <= selections["busy"] {
		t.Error("Healthy endpoint should be selected more than busy")
	}
	if selections["busy"] <= selections["warming"] {
		t.Error("Busy endpoint should be selected more than warming")
	}

	// All should be selected at least once
	for name, count := range selections {
		if count == 0 {
			t.Errorf("Endpoint %s was never selected", name)
		}
	}
}

func TestPrioritySelector_Select_PriorityOverridesWeight(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createPriorityEndpoint("low-healthy", 11434, domain.StatusHealthy, 100), // Lower priority but healthy
		createPriorityEndpoint("high-busy", 11435, domain.StatusBusy, 200),      // Higher priority but busy
	}

	// High priority endpoint should always be selected despite less weight
	for range 20 {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		if endpoint.Name != "high-busy" {
			t.Errorf("Expected higher priority endpoint, got %s", endpoint.Name)
		}
	}
}

func TestPrioritySelector_Select_OnlyRoutableEndpoints(t *testing.T) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createPriorityEndpoint("offline-high", 11434, domain.StatusOffline, 300),    // Highest priority but offline
		createPriorityEndpoint("healthy-low", 11435, domain.StatusHealthy, 100),     // Lower priority but healthy
		createPriorityEndpoint("unhealthy-med", 11436, domain.StatusUnhealthy, 200), // Medium priority but unhealthy
	}

	// Should select healthy endpoint despite lower priority
	for range 10 {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		if endpoint.Name != "healthy-low" {
			t.Errorf("Expected routable endpoint, got %s", endpoint.Name)
		}
	}
}
func TestPrioritySelector_ConnectionTracking(t *testing.T) {
	mockCollector := NewTestStatsCollector()
	selector := NewPrioritySelector(mockCollector)
	endpoint := createPriorityEndpoint("test", 11434, domain.StatusHealthy, 100)

	selector.IncrementConnections(endpoint)
	selector.IncrementConnections(endpoint)

	connectionStats := mockCollector.GetConnectionStats()
	count := connectionStats[endpoint.URL.String()]
	if count != 2 {
		t.Errorf("Expected 2 connections, got %d", count)
	}

	selector.DecrementConnections(endpoint)
	connectionStats = mockCollector.GetConnectionStats()
	count = connectionStats[endpoint.URL.String()]
	if count != 1 {
		t.Errorf("Expected 1 connection after decrement, got %d", count)
	}

	selector.DecrementConnections(endpoint)
	connectionStats = mockCollector.GetConnectionStats()
	count = connectionStats[endpoint.URL.String()]
