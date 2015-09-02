package unifier

import (
	"fmt"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// EndpointManager manages endpoint lifecycle and state
type EndpointManager struct {
	logger            logger.StyledLogger
	endpointStates    map[string]*domain.EndpointStateInfo
	endpointFailures  map[string]int
	circuitBreakers   map[string]*CircuitBreaker
	lastEndpointCheck map[string]time.Time
	config            Config
	mu                sync.RWMutex
}

// NewEndpointManager creates a new endpoint manager
func NewEndpointManager(config Config, logger logger.StyledLogger) *EndpointManager {
	return &EndpointManager{
		endpointStates:    make(map[string]*domain.EndpointStateInfo),
		endpointFailures:  make(map[string]int),
		circuitBreakers:   make(map[string]*CircuitBreaker),
		lastEndpointCheck: make(map[string]time.Time),
		config:            config,
		logger:            logger,
	}
}

// Reset forgets every endpoint's state in place. Replacing the manager instead would race
// with the readers that hold it without a lock.
func (m *EndpointManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.endpointStates = make(map[string]*domain.EndpointStateInfo)
	m.endpointFailures = make(map[string]int)
	m.circuitBreakers = make(map[string]*CircuitBreaker)
	m.lastEndpointCheck = make(map[string]time.Time)
}

// RecordFailure records an endpoint failure
func (m *EndpointManager) RecordFailure(endpointURL string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.endpointFailures[endpointURL]++
	failures := m.endpointFailures[endpointURL]

	// Update circuit breaker if enabled
	if m.config.CircuitBreaker.Enabled {
		cb := m.getOrCreateCircuitBreakerLocked(endpointURL)
		cb.RecordFailure()
	}

	// Update state
	if failures >= m.config.MaxConsecutiveFailures {
		m.markEndpointOfflineLocked(endpointURL, fmt.Sprintf("Too many failures: %v", err))
	} else {
		m.updateEndpointStateLocked(endpointURL, domain.EndpointStateDegraded, err.Error(), failures)
	}
}

// RecordSuccess records a successful endpoint interaction
func (m *EndpointManager) RecordSuccess(endpointURL string) {
