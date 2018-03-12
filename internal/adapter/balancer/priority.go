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
