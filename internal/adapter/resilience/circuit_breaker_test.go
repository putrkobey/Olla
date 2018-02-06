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
		t.Fatalf("LastAttemptNanos() = %d after RecordSuccess, want reset to 0", got)
	}
	for range 10 {
		if open, _ := b.IsOpen(testProbeStaleness); open {
			t.Fatal("closed breaker rejected a request")
		}
	}
}

// TestBreaker_HalfOpen_FailureReopens verifies a probe failure that crosses
// the threshold trips the breaker back open and blocks further traffic.
func TestBreaker_HalfOpen_FailureReopens(t *testing.T) {
	b := NewTripped(testConfig(1), testOpenDuration+time.Second) // single failure re-opens

	open, attempt := b.IsOpen(testProbeStaleness)
	if open {
		t.Fatal("expected the first call after timeout elapsed to admit a half-open probe")
	}

	b.RecordFailure(attempt)

	if !b.Tripped() {
		t.Fatal("expected the breaker to be open after a threshold-crossing RecordFailure")
	}
	if open, _ := b.IsOpen(testProbeStaleness); !open {
		t.Fatal("re-opened breaker admitted a request before its timeout elapsed again")
	}
}

// TestBreaker_HalfOpen_StaleProbeReleased is the regression test for a hung
// probe that never calls RecordSuccess/RecordFailure: it used to wedge the
// endpoint indefinitely, since IsOpen read lastAttempt without ever
// re-stamping it on the read path. Backdates lastAttempt past the staleness
// window directly (rather than sleeping in the test) to simulate a probe
// that has been in flight too long without resolving.
func TestBreaker_HalfOpen_StaleProbeReleased(t *testing.T) {
	b := NewTripped(testConfig(1000), testOpenDuration+time.Second)

	if open, _ := b.IsOpen(testProbeStaleness); open {
		t.Fatal("expected the first call to admit a half-open probe")
	}
	if open, _ := b.IsOpen(testProbeStaleness); !open {
		t.Fatal("a fresh in-flight probe must still block a second one")
	}

	b.SetLastAttemptNanos(time.Now().Add(-testProbeStaleness - time.Millisecond).UnixNano())

	if open, _ := b.IsOpen(testProbeStaleness); open {
		t.Fatal("a stale, never-resolved probe must not wedge the endpoint - a further probe should be admitted")
	}

	// The handover must re-stamp lastAttempt to now, not leave it stuck in
	// the past - otherwise every subsequent caller keeps reading a stale
	// timestamp and gets admitted too, defeating the single-flight gate.
	if open, _ := b.IsOpen(testProbeStaleness); !open {
		t.Fatal("the replacement probe just admitted must block the very next caller")
	}
}

// TestBreaker_HalfOpen_StaleProbeReleased_ConcurrentHandover races many
// goroutines against the same stale window to prove the last->now CAS
// handover admits exactly one replacement probe, not all of them.
func TestBreaker_HalfOpen_StaleProbeReleased_ConcurrentHandover(t *testing.T) {
	const goroutines = 200
	b := NewTripped(testConfig(1000), testOpenDuration+time.Second)
