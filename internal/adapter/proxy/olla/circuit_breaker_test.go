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

	// Until the probe resolves, every further caller must still be rejected.
	if open, _ := cb.IsOpen(HalfOpenStaleness); !open {
		t.Fatal("second probe was admitted before RecordSuccess/RecordFailure resolved the first")
	}

	// A failure resolves the in-flight probe and, since threshold is far away,
	// leaves the breaker in half-open - exactly one further probe should now
	// be admitted. attempt=0 because this test resolves the gate directly, not
	// via a captured probe token.
	cb.RecordFailure(0)

	admitted = 0
	ready.Add(goroutines)
	start.Add(1)
	done.Add(goroutines)
	for range goroutines {
		go func() {
			defer done.Done()
			ready.Done()
			start.Wait()
			if open, _ := cb.IsOpen(HalfOpenStaleness); !open {
				atomic.AddInt64(&admitted, 1)
			}
		}()
	}
	ready.Wait()
	start.Done()
	done.Wait()

	if admitted != 1 {
		t.Fatalf("after RecordFailure reset the gate: admitted = %d, want exactly 1", admitted)
	}
}

// TestCircuitBreaker_HalfOpen_SuccessCloses verifies RecordSuccess closes the
// breaker so subsequent traffic flows freely without going through the gate.
func TestCircuitBreaker_HalfOpen_SuccessCloses(t *testing.T) {
	cb := openCircuitBreaker(3)

	open, attempt := cb.IsOpen(HalfOpenStaleness)
	if open {
		t.Fatal("expected the first call after timeout elapsed to admit a half-open probe")
	}

	cb.RecordSuccess(attempt)

	if cb.Tripped() {
		t.Fatal("expected the breaker to be closed after RecordSuccess")
	}
	if got := cb.LastAttemptNanos(); got != 0 {
		t.Fatalf("lastAttempt = %d after RecordSuccess, want reset to 0", got)
	}
	for range 10 {
		if open, _ := cb.IsOpen(HalfOpenStaleness); open {
			t.Fatal("closed breaker rejected a request")
		}
	}
}

// TestCircuitBreaker_HalfOpen_StaleProbeReleased is the regression test for
// F2: a hung probe that never calls RecordSuccess/RecordFailure used to wedge
// the endpoint indefinitely, since lastAttempt only ever got reset by one of
// those two calls. Backdates lastAttempt past HalfOpenStaleness directly
// (rather than sleeping in the test) to simulate a probe that has been
// in-flight too long without resolving.
func TestCircuitBreaker_HalfOpen_StaleProbeReleased(t *testing.T) {
	cb := openCircuitBreaker(1000)

	if open, _ := cb.IsOpen(HalfOpenStaleness); open {
		t.Fatal("expected the first call to admit a half-open probe")
	}
	if open, _ := cb.IsOpen(HalfOpenStaleness); !open {
		t.Fatal("a fresh in-flight probe must still block a second one")
	}

	// Simulate the first probe hanging well past the staleness window without
	// ever resolving.
	cb.SetLastAttemptNanos(time.Now().Add(-HalfOpenStaleness - time.Millisecond).UnixNano())

	if open, _ := cb.IsOpen(HalfOpenStaleness); open {
		t.Fatal("a stale, never-resolved probe must not wedge the endpoint - a further probe should be admitted")
	}

	// The handover must re-stamp lastAttempt to now, not leave it stuck in the
	// past - otherwise every subsequent caller keeps reading a stale timestamp
	// and gets admitted too, defeating the single-flight gate entirely.
	if open, _ := cb.IsOpen(HalfOpenStaleness); !open {
		t.Fatal("the replacement probe just admitted must block the very next caller")
	}
}

// TestCircuitBreaker_HalfOpen_StaleProbeReleased_ConcurrentHandover races many
// goroutines against the same stale window to prove the last->now CAS handover
// admits exactly one replacement probe, not all of them. Before the fix, every
// goroutine that observed the stale window read the same never-updated
// lastAttempt and was admitted, regardless of how many arrived.
func TestCircuitBreaker_HalfOpen_StaleProbeReleased_ConcurrentHandover(t *testing.T) {
	const goroutines = 200
	cb := openCircuitBreaker(1000)

	// Win the initial half-open slot and immediately backdate it past the
	// staleness window, so every goroutine below races the handover CAS.
	if open, _ := cb.IsOpen(HalfOpenStaleness); open {
		t.Fatal("expected the first call to admit a half-open probe")
	}
	cb.SetLastAttemptNanos(time.Now().Add(-HalfOpenStaleness - time.Millisecond).UnixNano())

	var admitted int64
	var ready sync.WaitGroup
	var start sync.WaitGroup
	var done sync.WaitGroup

	ready.Add(goroutines)
	start.Add(1)
	done.Add(goroutines)

	for range goroutines {
		go func() {
