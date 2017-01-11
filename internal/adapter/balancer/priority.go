package balancer

import (
	"context"
	"errors"
	"math/rand"
	"sort"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// PrioritySelector implements priority-based endpoint selection with connection tracking
type PrioritySelector struct {
	statsCollector ports.StatsCollector
}

// NewPrioritySelector creates a new priority-based endpoint selector
func NewPrioritySelector(statsCollector ports.StatsCollector) *PrioritySelector {
	return &PrioritySelector{
		statsCollector: statsCollector,
	}
}

// Name returns the name of the selection strategy
func (p *PrioritySelector) Name() string {
	return DefaultBalancerPriority
}

// Select chooses the highest priority routable endpoint with weighted selection for non-healthy statuses
func (p *PrioritySelector) Select(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no endpoints available")
	}

	// Filter only routable endpoints
	routable := make([]*domain.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Status.IsRoutable() {
			routable = append(routable, endpoint)
		}
	}

	if len(routable) == 0 {
		return nil, errors.New("no routable endpoints available")
	}

	// Sort by priority (highest first)
	sort.Slice(routable, func(i, j int) bool {
		return routable[i].Priority > routable[j].Priority
	})

	// Get the highest priority tier
	highestPriority := routable[0].Priority
	// Pre-allocate with capacity for all routable endpoints (worst case all have same priority)
	highestPriorityEndpoints := make([]*domain.Endpoint, 0, len(routable))

	for _, endpoint := range routable {
		if endpoint.Priority == highestPriority {
			highestPriorityEndpoints = append(highestPriorityEndpoints, endpoint)
		} else {
			break // Since sorted, we can break early
		}
	}

	// If only one endpoint at highest priority, return it
	if len(highestPriorityEndpoints) == 1 {
		return highestPriorityEndpoints[0], nil
	}

