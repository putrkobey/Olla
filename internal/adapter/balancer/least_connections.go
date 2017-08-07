package balancer

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// LeastConnectionsSelector implements a load balancer that selects the endpoint with the least number of active connections.
type LeastConnectionsSelector struct {
	statsCollector ports.StatsCollector
	// cursor rotates the tie-break among endpoints sharing the minimum
	// connection count. A strict less-than comparison alone always keeps the
	// first tied candidate in registration order, so a recovered endpoint
	// tied with an already-healthy one would never receive traffic under
	// sequential requests - it's permanently shadowed by whichever endpoint
	// happens to sort first. Plain atomic counter, no mutex: Select is a
	// per-request hot path and the cursor only needs to move, not be exact.
	cursor atomic.Uint64
}

func NewLeastConnectionsSelector(statsCollector ports.StatsCollector) *LeastConnectionsSelector {
	return &LeastConnectionsSelector{
		statsCollector: statsCollector,
	}
}

func (l *LeastConnectionsSelector) Name() string {
	return DefaultBalancerLeastConnections
}

func (l *LeastConnectionsSelector) Select(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no endpoints available")
	}

	routable := make([]*domain.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Status.IsRoutable() {
			routable = append(routable, endpoint)
		}
