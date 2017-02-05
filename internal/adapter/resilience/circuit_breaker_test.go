package resilience

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testOpenDuration = 30 * time.Second
const testProbeStaleness = 5 * time.Second

func testConfig(threshold int) Config {
	return Config{FailureThreshold: threshold, OpenDuration: testOpenDuration}
}

// TestBreaker_ClosedByDefault verifies a fresh Breaker starts closed and
// admits traffic without any half-open gating.
func TestBreaker_ClosedByDefault(t *testing.T) {
	b := New(testConfig(3))
	if open, attempt := b.IsOpen(testProbeStaleness); open || attempt != 0 {
		t.Fatalf("IsOpen() = (%v, %d) on a fresh breaker, want (false, 0)", open, attempt)
	}
}

// TestBreaker_TripsAtThreshold verifies the breaker opens exactly when
// consecutive failures reach the configured threshold, not before.
func TestBreaker_TripsAtThreshold(t *testing.T) {
	b := New(testConfig(3))

	b.RecordFailure(0)
	b.RecordFailure(0)
	if b.Tripped() {
		t.Fatal("breaker tripped before reaching the failure threshold")
	}

	b.RecordFailure(0)
	if !b.Tripped() {
		t.Fatal("breaker did not trip after reaching the failure threshold")
	}
	if open, _ := b.IsOpen(testProbeStaleness); !open {
		t.Fatal("expected IsOpen to block traffic immediately after tripping")
	}
}

// TestBreaker_SuccessResetsFailureCount verifies a success on the closed path
// resets the consecutive-failure counter, matching a standard circuit
// breaker's "consecutive" semantics rather than a rolling total.
func TestBreaker_SuccessResetsFailureCount(t *testing.T) {
	b := New(testConfig(3))

	b.RecordFailure(0)
	b.RecordFailure(0)
	b.RecordSuccess(0)
	if got := b.Failures(); got != 0 {
		t.Fatalf("Failures() = %d after a success, want 0", got)
	}

	b.RecordFailure(0)
	b.RecordFailure(0)
	if b.Tripped() {
		t.Fatal("breaker tripped despite the failure count having been reset by the intervening success")
	}
}

// TestBreaker_StaysOpenUntilRecoveryTimeout verifies a tripped breaker keeps
// rejecting every call, without ever admitting a half-open probe, until
// OpenDuration has elapsed since the last failure.
func TestBreaker_StaysOpenUntilRecoveryTimeout(t *testing.T) {
	b := NewTripped(testConfig(1), testOpenDuration/2) // tripped, but only half the recovery window has passed

	open, attempt := b.IsOpen(testProbeStaleness)
	if !open || attempt != 0 {
		t.Fatalf("IsOpen() = (%v, %d) before the recovery timeout elapsed, want (true, 0)", open, attempt)
	}
}

// TestBreaker_HalfOpen_SingleFlight is the regression test for the burst bug:
// once the recovery timeout elapses, only exactly one concurrent probe should
// be admitted until RecordSuccess/RecordFailure resolves it - not every
// caller that happens to observe the elapsed timeout simultaneously.
func TestBreaker_HalfOpen_SingleFlight(t *testing.T) {
	const goroutines = 200
	b := NewTripped(testConfig(1000), testOpenDuration+time.Second) // high threshold so a probe failure doesn't reopen mid-test

	var admitted int64
	var ready, start, done sync.WaitGroup
	ready.Add(goroutines)
	start.Add(1)
	done.Add(goroutines)

	for range goroutines {
		go func() {
			defer done.Done()
			ready.Done()
			start.Wait()
			if open, _ := b.IsOpen(testProbeStaleness); !open {
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

	if open, _ := b.IsOpen(testProbeStaleness); !open {
		t.Fatal("second probe was admitted before RecordSuccess/RecordFailure resolved the first")
	}
}

// TestBreaker_HalfOpen_SuccessCloses verifies RecordSuccess closes the
// breaker so subsequent traffic flows freely without going through the gate.
func TestBreaker_HalfOpen_SuccessCloses(t *testing.T) {
	b := NewTripped(testConfig(3), testOpenDuration+time.Second)

	open, attempt := b.IsOpen(testProbeStaleness)
	if open {
		t.Fatal("expected the first call after timeout elapsed to admit a half-open probe")
	}

	b.RecordSuccess(attempt)

	if b.Tripped() {
		t.Fatal("expected the breaker to be closed after RecordSuccess")
	}
	if got := b.LastAttemptNanos(); got != 0 {
