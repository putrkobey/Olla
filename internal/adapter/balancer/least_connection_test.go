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
