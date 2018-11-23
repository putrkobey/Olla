package unifier

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenDuration:     100 * time.Millisecond,
		HalfOpenRequests: 1,
	}

	cb := NewCircuitBreaker(config)
	assert.Equal(t, CircuitClosed, cb.GetState())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_OpenOnFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenDuration:     100 * time.Millisecond,
		HalfOpenRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	// Record failures
	for range 2 {
		cb.RecordFailure()
		assert.Equal(t, CircuitClosed, cb.GetState())
		assert.True(t, cb.Allow())
	}

	// Third failure should open the circuit
	cb.RecordFailure()
	assert.Equal(t, CircuitOpen, cb.GetState())
	assert.False(t, cb.Allow())
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 1,
		SuccessThreshold: 2,
		OpenDuration:     50 * time.Millisecond,
		HalfOpenRequests: 2,
	}

	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.RecordFailure()
	assert.Equal(t, CircuitOpen, cb.GetState())
	assert.False(t, cb.Allow())

	// Wait for open duration
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open
	assert.True(t, cb.Allow())
	assert.Equal(t, CircuitHalfOpen, cb.GetState())

	// Should allow limited requests
	assert.True(t, cb.Allow())
	assert.False(t, cb.Allow()) // Exceeds half-open limit
}

func TestCircuitBreaker_CloseOnSuccess(t *testing.T) {
	config := CircuitBreakerConfig{
