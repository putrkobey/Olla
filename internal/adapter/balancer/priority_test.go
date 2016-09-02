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
