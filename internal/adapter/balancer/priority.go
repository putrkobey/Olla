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
