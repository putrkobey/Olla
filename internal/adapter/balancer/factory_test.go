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
