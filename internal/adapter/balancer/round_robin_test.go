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
			t.Fatalf("Select %d failed: %v", i, err)
		}
		if endpoint.Name != expected {
			t.Errorf("Selection %d: expected %s, got %s", i, expected, endpoint.Name)
		}
	}
}

func TestRoundRobinSelector_Select_OnlyRoutableEndpoints(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createRoundRobinEndpoint("offline", 11434, domain.StatusOffline),     // Not routable
		createRoundRobinEndpoint("healthy", 11435, domain.StatusHealthy),     // Routable
		createRoundRobinEndpoint("unhealthy", 11436, domain.StatusUnhealthy), // Not routable
		createRoundRobinEndpoint("busy", 11437, domain.StatusBusy),           // Routable
		createRoundRobinEndpoint("warming", 11438, domain.StatusWarming),     // Routable
	}

	selections := make(map[string]int)
	routableEndpoints := []string{"healthy", "busy", "warming"}

	// Run enough selections to cycle through routable endpoints multiple times
	for range 15 {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		selections[endpoint.Name]++
	}

	// Should never select non-routable endpoints
	if selections["offline"] > 0 {
		t.Error("Offline endpoint was selected")
	}
	if selections["unhealthy"] > 0 {
		t.Error("Unhealthy endpoint was selected")
	}

	// Should select all routable endpoints
	for _, name := range routableEndpoints {
		if selections[name] == 0 {
			t.Errorf("Routable endpoint %s was never selected", name)
		}
	}

	// Should distribute evenly (15 selections / 3 routable = 5 each)
	for _, name := range routableEndpoints {
		if selections[name] != 5 {
			t.Errorf("Expected 5 selections for %s, got %d", name, selections[name])
		}
	}
}

func TestRoundRobinSelector_Select_CounterOverflow(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createRoundRobinEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createRoundRobinEndpoint("endpoint-2", 11435, domain.StatusHealthy),
	}

	// Set counter to a high value near overflow
	selector.counter = ^uint64(0) - 5 // Near uint64 max

	// Should handle overflow gracefully
	for range 10 {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed with high counter: %v", err)
		}
		if endpoint == nil {
			t.Fatal("Got nil endpoint with high counter")
		}
	}

	// Verify it still works after overflow
	endpoint, err := selector.Select(ctx, endpoints)
	if err != nil {
		t.Fatalf("Select failed after overflow: %v", err)
	}
	if endpoint == nil {
		t.Fatal("Got nil endpoint after overflow")
	}
}

func TestRoundRobinSelector_ConnectionTracking(t *testing.T) {
	mockCollector := NewTestStatsCollector()
	selector := NewRoundRobinSelector(mockCollector)
	endpoint := createRoundRobinEndpoint("test", 11434, domain.StatusHealthy)

	// Test increment
	selector.IncrementConnections(endpoint)
	selector.IncrementConnections(endpoint)

	// Verify count from stats collector
	connectionStats := mockCollector.GetConnectionStats()
	count := connectionStats[endpoint.URL.String()]
	if count != 2 {
		t.Errorf("Expected 2 connections, got %d", count)
	}

	// Test decrement
	selector.DecrementConnections(endpoint)
	connectionStats = mockCollector.GetConnectionStats()
	count = connectionStats[endpoint.URL.String()]
	if count != 1 {
		t.Errorf("Expected 1 connection after decrement, got %d", count)
	}

	// Connection tracking doesn't affect selection in round-robin,
	// but we should verify it doesn't cause errors
	endpoints := []*domain.Endpoint{endpoint}
	selected, err := selector.Select(context.Background(), endpoints)
	if err != nil {
		t.Errorf("Selection failed after connection tracking: %v", err)
	}
	if selected.Name != "test" {
		t.Error("Wrong endpoint selected after connection tracking")
	}
}

func TestRoundRobinSelector_DecrementBelowZero(t *testing.T) {
	mockCollector := NewTestStatsCollector()
	selector := NewRoundRobinSelector(mockCollector)
	endpoint := createRoundRobinEndpoint("test", 11434, domain.StatusHealthy)

	// Try to decrement without any connections
	selector.DecrementConnections(endpoint)

	// Should handle gracefully - verify no panic and count stays at 0
	connectionStats := mockCollector.GetConnectionStats()
	count := connectionStats[endpoint.URL.String()]
	if count != 0 {
		t.Errorf("Expected 0 connections after decrement below zero, got %d", count)
	}

	endpoints := []*domain.Endpoint{endpoint}
	_, err := selector.Select(context.Background(), endpoints)
	if err != nil {
		t.Errorf("Selection failed after decrement below zero: %v", err)
	}
}

func TestRoundRobinSelector_ConcurrentAccess(t *testing.T) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createRoundRobinEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createRoundRobinEndpoint("endpoint-2", 11435, domain.StatusHealthy),
		createRoundRobinEndpoint("endpoint-3", 11436, domain.StatusHealthy),
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)
	selections := make(chan string, 300)

	// Concurrent selections
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				endpoint, err := selector.Select(ctx, endpoints)
				if err != nil {
					errors <- err
					return
				}
				selections <- endpoint.Name
			}
		}()
	}

	// Concurrent connection tracking
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			endpoint := endpoints[id%len(endpoints)]
			for range 5 {
				selector.IncrementConnections(endpoint)
				selector.DecrementConnections(endpoint)
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	close(selections)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}

	// Verify all selections were valid
	selectionCounts := make(map[string]int)
	totalSelections := 0
	for selection := range selections {
		selectionCounts[selection]++
		totalSelections++

		// Verify it's a valid endpoint name
		valid := false
		for _, endpoint := range endpoints {
			if endpoint.Name == selection {
				valid = true
				break
			}
