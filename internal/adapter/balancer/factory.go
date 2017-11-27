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
