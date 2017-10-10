package balancer

import (
	"fmt"
	"sync"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

const DefaultBalancerPriority = "priority"
const DefaultBalancerRoundRobin = "round-robin"
const DefaultBalancerLeastConnections = "least-connections"

type Factory struct {
	creators       map[string]func(ports.StatsCollector) domain.EndpointSelector
	statsCollector ports.StatsCollector
	mu             sync.RWMutex
}

func NewFactory(statsCollector ports.StatsCollector) *Factory {
	factory := &Factory{
		creators:       make(map[string]func(ports.StatsCollector) domain.EndpointSelector),
		statsCollector: statsCollector,
	}

	factory.Register(DefaultBalancerPriority, func(collector ports.StatsCollector) domain.EndpointSelector {
		return NewPrioritySelector(collector)
	})
	factory.Register(DefaultBalancerRoundRobin, func(collector ports.StatsCollector) domain.EndpointSelector {
		return NewRoundRobinSelector(collector)
	})
	factory.Register(DefaultBalancerLeastConnections, func(collector ports.StatsCollector) domain.EndpointSelector {
		return NewLeastConnectionsSelector(collector)
	})

	return factory
}

func (f *Factory) Register(name string, creator func(ports.StatsCollector) domain.EndpointSelector) {
