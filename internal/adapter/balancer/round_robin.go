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

