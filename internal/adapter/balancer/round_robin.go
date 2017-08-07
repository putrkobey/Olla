package balancer

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

type RoundRobinSelector struct {
	statsCollector ports.StatsCollector
	counter        uint64
}

func NewRoundRobinSelector(statsCollector ports.StatsCollector) *RoundRobinSelector {
	return &RoundRobinSelector{
		statsCollector: statsCollector,
	}
}

func (r *RoundRobinSelector) Name() string {
	return DefaultBalancerRoundRobin
}

// Select chooses endpoints in a round-robin fashion, filtering out non-routable endpoints
func (r *RoundRobinSelector) Select(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no endpoints available")
	}

	routable := make([]*domain.Endpoint, 0, len(endpoints))
