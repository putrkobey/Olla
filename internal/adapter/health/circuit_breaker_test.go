package health

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"

	"github.com/putrkobey/Olla/internal/adapter/resilience"
)

const testEndpoint = "http://endpoint-under-test"

// testCheckTimeout mirrors config.go's own default for Endpoint.CheckTimeout
// (2s), so probeStaleness(testCheckTimeout) == 5s (2*2s + 1s margin) in these
// tests - the same budget a real endpoint would get.
const testCheckTimeout = 2 * time.Second

// openCircuitBreaker builds a CircuitBreaker with a single endpoint already
// tripped into the open state, its recovery timeout already elapsed so the
// next IsOpen() call is eligible to transition into half-open. Mirrors
// proxy/olla/circuit_breaker_test.go's openCircuitBreaker helper.
func openCircuitBreaker(threshold int) *CircuitBreaker {
	cb := &CircuitBreaker{
		endpoints:        xsync.NewMap[string, *resilience.Breaker](),
		failureThreshold: threshold,
		timeout:          DefaultCircuitBreakerTimeout,
	}
	cb.endpoints.Store(testEndpoint, resilience.NewTripped(resilience.Config{
		FailureThreshold: threshold,
		OpenDuration:     DefaultCircuitBreakerTimeout,
	}, DefaultCircuitBreakerTimeout+time.Second))
	return cb
}

// TestProbeStaleness_DerivedFromCheckTimeout pins the window-derivation fix:
// the half-open staleness window must scale with the endpoint's own
// CheckTimeout (checker.go wraps each probe attempt, including internal
// retries, in a CheckTimeout*2 context), not a single fixed constant that
// could be tighter than a legitimately slow probe's real budget.
func TestProbeStaleness_DerivedFromCheckTimeout(t *testing.T) {
	tests := []struct {
		name         string
		checkTimeout time.Duration
		want         time.Duration
	}{
		{"2s config default", 2 * time.Second, 5 * time.Second},
		{"scales with a larger configured timeout", 10 * time.Second, 21 * time.Second},
		{"zero falls back to the config default", 0, 5 * time.Second},
		{"negative falls back to the config default", -time.Second, 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := probeStaleness(tt.checkTimeout)
			if got != tt.want {
				t.Errorf("probeStaleness(%v) = %v, want %v", tt.checkTimeout, got, tt.want)
			}
		})
	}
}

// TestCircuitBreaker_HalfOpen_SlowButAliveProbeWithinBudget_NoHandover is the
// regression test for the review finding: a fixed 1s staleness window was
// tighter than checker.go's real CheckTimeout*2 probe budget (5s for the 2s
// config default), so a legitimately slow-but-alive probe could trigger a
// spurious stale-handover and admit a second concurrent probe against a
// backend that was merely slow, not hung. A probe backdated to within the
// derived window must still block a second caller.
func TestCircuitBreaker_HalfOpen_SlowButAliveProbeWithinBudget_NoHandover(t *testing.T) {
	cb := openCircuitBreaker(1000)

	if open, _ := cb.IsOpen(testEndpoint, testCheckTimeout); open {
		t.Fatal("expected the first call to admit a half-open probe")
	}

	// Backdate to 4s ago - within the 5s budget (2*2s + 1s margin) for a 2s
	// CheckTimeout, i.e. still legitimately in flight, not hung.
	state, _ := cb.endpoints.Load(testEndpoint)
	backdated := time.Now().Add(-4 * time.Second).UnixNano()
	state.SetLastAttemptNanos(backdated)

	if open, _ := cb.IsOpen(testEndpoint, testCheckTimeout); !open {
		t.Fatal("a probe still within its CheckTimeout-derived budget must not be superseded - a second caller must be rejected")
	}

	// Sanity: the in-budget probe's stamp is still authoritative - IsOpen must
	// not have performed a handover CAS against it.
	if got := state.LastAttemptNanos(); got != backdated {
		t.Fatalf("lastAttempt changed even though the probe was within budget: got %d, want unchanged %d", got, backdated)
	}
}

// TestCircuitBreaker_HalfOpen_HungProbeBeyondBudget_TriggersHandover is the
// inverse: a probe backdated PAST the CheckTimeout-derived budget is
// legitimately hung, and a replacement must be admitted.
func TestCircuitBreaker_HalfOpen_HungProbeBeyondBudget_TriggersHandover(t *testing.T) {
	cb := openCircuitBreaker(1000)

	if open, _ := cb.IsOpen(testEndpoint, testCheckTimeout); open {
		t.Fatal("expected the first call to admit a half-open probe")
	}

	// Backdate past the 5s budget.
	state, _ := cb.endpoints.Load(testEndpoint)
	state.SetLastAttemptNanos(time.Now().Add(-5*time.Second - time.Millisecond).UnixNano())

	open, replacementAttempt := cb.IsOpen(testEndpoint, testCheckTimeout)
