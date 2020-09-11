package unifier

import (
	"sync"
	"sync/atomic"
	"time"
)

type CircuitBreakerState int32

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker prevents cascade failures by blocking requests to failing endpoints.
// It transitions between closed (normal), open (blocking), and half-open (testing) states.
type CircuitBreaker struct {
	config           CircuitBreakerConfig
	lastFailureTime  atomic.Int64
	mu               sync.RWMutex
	state            atomic.Int32
	failures         atomic.Int32
	successes        atomic.Int32
	halfOpenRequests atomic.Int32
}

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		config: config,
	}
	cb.state.Store(int32(CircuitClosed))
	return cb
}

func (cb *CircuitBreaker) Allow() bool {
	if !cb.config.Enabled {
		return true
	}

	state := CircuitBreakerState(cb.state.Load())

	switch state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		lastFailure := time.Unix(0, cb.lastFailureTime.Load())
		if time.Since(lastFailure) > cb.config.OpenDuration {
			cb.transitionToHalfOpen()
			return cb.allowHalfOpen()
		}
		return false

	case CircuitHalfOpen:
		return cb.allowHalfOpen()

