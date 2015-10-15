package balancer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/ports"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory(NewTestStatsCollector())

	if factory == nil {
		t.Fatal("NewFactory returned nil")
	}

	if factory.creators == nil {
		t.Error("Factory creators map not initialised")
	}

	// Test default strategies are registered
	expectedStrategies := []string{DefaultBalancerPriority, DefaultBalancerRoundRobin, DefaultBalancerLeastConnections}
	available := factory.GetAvailableStrategies()

	if len(available) != len(expectedStrategies) {
		t.Errorf("Expected %d default strategies, got %d", len(expectedStrategies), len(available))
	}

	for _, expected := range expectedStrategies {
		found := false
		for _, actual := range available {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected strategy %q not found in available strategies", expected)
		}
	}
}

func TestFactory_Create_DefaultStrategies(t *testing.T) {
	factory := NewFactory(NewTestStatsCollector())

	testCases := []struct {
		name          string
		expectedType  string
		shouldSucceed bool
	}{
		{DefaultBalancerPriority, DefaultBalancerPriority, true},
		{DefaultBalancerRoundRobin, DefaultBalancerRoundRobin, true},
		{DefaultBalancerLeastConnections, DefaultBalancerLeastConnections, true},
		{"unknown-strategy", "", false},
		{"", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selector, err := factory.Create(tc.name)

			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Create(%q) failed: %v", tc.name, err)
				}
				if selector == nil {
					t.Fatal("Create returned nil selector")
				}
				if selector.Name() != tc.expectedType {
					t.Errorf("Expected selector name %q, got %q", tc.expectedType, selector.Name())
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for unknown strategy %q", tc.name)
				}
				if selector != nil {
					t.Error("Expected nil selector for unknown strategy")
				}
			}
		})
	}
}

func TestFactory_Register_CustomStrategy(t *testing.T) {
	factory := NewFactory(NewTestStatsCollector())
	customName := "custom"

	// Mock custom selector
	customCreator := func(collector ports.StatsCollector) domain.EndpointSelector {
		return &mockEndpointSelector{
			name:           customName,
			statsCollector: collector,
		}
	}

	// Register custom strategy
	factory.Register(customName, customCreator)

	// Verify it's in available strategies
	available := factory.GetAvailableStrategies()
	found := false
	for _, strategy := range available {
		if strategy == customName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Custom strategy not found in available strategies")
	}

	// Test creating custom strategy
