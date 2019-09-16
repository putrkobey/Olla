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

