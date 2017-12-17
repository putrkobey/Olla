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

const balancerName = "least-connections"

func TestNewLeastConnectionsSelector(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())

	if selector == nil {
		t.Fatal("NewLeastConnectionsSelector returned nil")
	}

	if selector.statsCollector.GetConnectionStats() == nil {
		t.Error("Connections map not initialised")
	}

	if selector.Name() != balancerName {
		t.Errorf("Expected name '%s', got %q", balancerName, selector.Name())
	}
}

func TestLeastConnectionsSelector_Select_NoEndpoints(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoint, err := selector.Select(ctx, []*domain.Endpoint{})
	if err == nil {
		t.Error("Expected error for empty endpoints")
	}
	if endpoint != nil {
		t.Error("Expected nil endpoint for empty slice")
	}
}

func TestLeastConnectionsSelector_Select_NoRoutableEndpoints(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	ctx := context.Background()

	// Create endpoints with non-routable statuses
	endpoints := []*domain.Endpoint{
		createTestEndpoint("endpoint-1", 11434, domain.StatusOffline),
		createTestEndpoint("endpoint-2", 11435, domain.StatusUnhealthy),
		createTestEndpoint("endpoint-3", 11436, domain.StatusUnknown),
	}

	endpoint, err := selector.Select(ctx, endpoints)
	if err == nil {
		t.Error("Expected error for no routable endpoints")
	}
	if endpoint != nil {
		t.Error("Expected nil endpoint for no routable endpoints")
	}
}

func TestLeastConnectionsSelector_Select_SingleEndpoint(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createTestEndpoint("endpoint-1", 11434, domain.StatusHealthy),
	}

	endpoint, err := selector.Select(ctx, endpoints)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if endpoint == nil {
		t.Fatal("Expected endpoint, got nil")
	}
	if endpoint.Name != "endpoint-1" {
		t.Errorf("Expected endpoint-1, got %s", endpoint.Name)
	}
}

func TestLeastConnectionsSelector_Select_MultipleEndpoints(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createTestEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createTestEndpoint("endpoint-2", 11435, domain.StatusHealthy),
		createTestEndpoint("endpoint-3", 11436, domain.StatusBusy),
	}

	// First selection should pick first endpoint (all have 0 connections)
	endpoint, err := selector.Select(ctx, endpoints)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if endpoint.Name != "endpoint-1" {
		t.Errorf("Expected endpoint-1 first, got %s", endpoint.Name)
	}

	// Add connections to first endpoint
	selector.IncrementConnections(endpoints[0])
	selector.IncrementConnections(endpoints[0])

	// Add one connection to second endpoint
	selector.IncrementConnections(endpoints[1])

	// Third endpoint should now be selected (0 connections vs 2 and 1)
	endpoint, err = selector.Select(ctx, endpoints)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if endpoint.Name != "endpoint-3" {
		t.Errorf("Expected endpoint-3 (least connections), got %s", endpoint.Name)
	}
}

func TestLeastConnectionsSelector_Select_OnlyRoutableEndpoints(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createTestEndpoint("offline", 11434, domain.StatusOffline),
		createTestEndpoint("healthy", 11435, domain.StatusHealthy),
		createTestEndpoint("unhealthy", 11436, domain.StatusUnhealthy),
		createTestEndpoint("busy", 11437, domain.StatusBusy),
		createTestEndpoint("warming", 11438, domain.StatusWarming),
	}

	// Should only consider healthy, busy, and warming endpoints
	selectedNames := make(map[string]int)
	connectionCounts := make(map[string]int)

	for i := range 50 {
		endpoint, err := selector.Select(ctx, endpoints)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		selectedNames[endpoint.Name]++

		// Simulate connection tracking to ensure load balancing
		selector.IncrementConnections(endpoint)
		connectionCounts[endpoint.Name]++

		// Occasionally complete some connections to create variation
		if i%5 == 0 && connectionCounts[endpoint.Name] > 1 {
			selector.DecrementConnections(endpoint)
			connectionCounts[endpoint.Name]--
		}
	}

	// Should never select offline or unhealthy
	if selectedNames["offline"] > 0 {
		t.Error("Offline endpoint was selected")
	}
	if selectedNames["unhealthy"] > 0 {
		t.Error("Unhealthy endpoint was selected")
	}

	// Should select routable endpoints
	if selectedNames["healthy"] == 0 {
		t.Error("Healthy endpoint was never selected")
	}
	if selectedNames["busy"] == 0 {
		t.Error("Busy endpoint was never selected")
	}
	if selectedNames["warming"] == 0 {
		t.Error("Warming endpoint was never selected")
	}
}

func TestLeastConnectionsSelector_ConnectionTracking(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoint := createTestEndpoint("test", 11434, domain.StatusHealthy)

	// Test increment
	selector.IncrementConnections(endpoint)
	selector.IncrementConnections(endpoint)

	// Verify internal state by selecting from multiple endpoints
	endpoints := []*domain.Endpoint{
		endpoint,
		createTestEndpoint("test2", 11435, domain.StatusHealthy),
	}

	selected, _ := selector.Select(ctx, endpoints)
	// Should select test2 since it has 0 connections vs test with 2
	if selected.Name != "test2" {
		t.Error("Expected endpoint with fewer connections to be selected")
	}

	// Test decrement
	selector.DecrementConnections(endpoint)
	selected, _ = selector.Select(ctx, endpoints)
	// Now test has 1 connection, test2 has 0, so test2 should still be selected
	if selected.Name != "test2" {
		t.Error("Expected endpoint with fewer connections after decrement")
	}

	// Decrement again
	selector.DecrementConnections(endpoint)
	// Both should have 0 connections now, so either could be selected
	// We just verify no error occurs
	_, err := selector.Select(ctx, endpoints)
	if err != nil {
		t.Errorf("Selection failed after decrement to zero: %v", err)
	}
}

func TestLeastConnectionsSelector_DecrementBelowZero(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	endpoint := createTestEndpoint("test", 11434, domain.StatusHealthy)

	// Try to decrement without any connections
	selector.DecrementConnections(endpoint)

	// Should handle gracefully - verify by checking selection behavior
	endpoints := []*domain.Endpoint{
		endpoint,
		createTestEndpoint("test2", 11435, domain.StatusHealthy),
	}

	selector.IncrementConnections(endpoints[1])
	selected, _ := selector.Select(context.Background(), endpoints)

	// First endpoint should be selected (0 connections vs 1)
	if selected.Name != "test" {
		t.Error("Decrement below zero not handled correctly")
	}
}

func TestLeastConnectionsSelector_ConcurrentAccess(t *testing.T) {
	selector := NewLeastConnectionsSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := []*domain.Endpoint{
		createTestEndpoint("endpoint-1", 11434, domain.StatusHealthy),
		createTestEndpoint("endpoint-2", 11435, domain.StatusHealthy),
		createTestEndpoint("endpoint-3", 11436, domain.StatusHealthy),
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent selections
	for range 20 {
