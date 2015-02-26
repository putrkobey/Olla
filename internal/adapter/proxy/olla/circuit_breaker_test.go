package olla

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/health"
	"github.com/putrkobey/Olla/internal/adapter/resilience"
)

// openCircuitBreaker builds a circuitBreaker already tripped into the open state,
// with its recovery timeout already elapsed so the next IsOpen() call is eligible
// to transition into half-open.
func openCircuitBreaker(threshold int64) *circuitBreaker {
	return resilience.NewTripped(resilience.Config{
		FailureThreshold: int(threshold),
		OpenDuration:     health.DefaultCircuitBreakerTimeout,
	}, health.DefaultCircuitBreakerTimeout+time.Second)
}

// TestCircuitBreaker_HalfOpen_SingleFlight is the regression test for the burst
// bug: once state CASes open -> half-open, every concurrent caller used to see
// state != 1 and pass straight through, flooding the recovering backend. Only
// exactly one concurrent probe should be admitted until RecordSuccess/RecordFailure
// resolves it.
func TestCircuitBreaker_HalfOpen_SingleFlight(t *testing.T) {
	const goroutines = 200
	cb := openCircuitBreaker(1000) // high threshold so a probe failure doesn't reopen mid-test

	var admitted int64
	var ready sync.WaitGroup
	var start sync.WaitGroup
	var done sync.WaitGroup

	ready.Add(goroutines)
	start.Add(1)
	done.Add(goroutines)

	for range goroutines {
		go func() {
			defer done.Done()
			ready.Done()
			start.Wait() // release all goroutines at once to maximise contention
			if open, _ := cb.IsOpen(HalfOpenStaleness); !open {
				atomic.AddInt64(&admitted, 1)
			}
		}()
	}

	ready.Wait()
	start.Done()
	done.Wait()

	if admitted != 1 {
		t.Fatalf("admitted = %d concurrent probes in half-open, want exactly 1", admitted)
	}

