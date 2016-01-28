// Package resilience holds the shared per-endpoint circuit breaker state
// machine used by both proxy/olla (proxied inference requests) and
// adapter/health (health-check probes). The two call sites hand-rolled
// functionally identical breakers - closed/open/half-open, single-flight
// half-open probe gating, stale-handover CAS, ABA-safe attempt tokens - and
// converged on the same bug fixes independently, with one lagging the other
// until a fourth review caught the gap. Keeping ONE implementation means a
// correctness fix only has to land once.
//
// This package deliberately knows nothing about endpoints, URLs, maps,
// cleanup sweeps or event emission - those are call-site concerns and stay in
// proxy/olla and adapter/health respectively. Neither of those two packages
// may import the other, so the shared type lives here instead, one level up,
// with no dependency on either.
package resilience

import (
	"sync/atomic"
	"time"
)

// Config configures a Breaker's failure threshold and open-state duration.
//
// The half-open single-flight staleness window (how long an in-flight probe
// is trusted before a replacement is admitted) is deliberately NOT part of
// Config - it is supplied per IsOpen call instead. The two adapters derive it
// differently: adapter/health computes it per endpoint from that endpoint's
// own CheckTimeout (one CircuitBreaker instance is shared across every
// endpoint, keyed by health-check URL, so the window has to vary per call),
// while proxy/olla uses one fixed constant sized to inference response
// budgets, applied uniformly (one Breaker instance per endpoint, so the
// constant could live on the Config there too, but taking it as a call
// parameter keeps both adapters' derivation logic entirely their own).
type Config struct {
	FailureThreshold int
	OpenDuration     time.Duration
}

// Breaker states. half-open is a real, persisted state (not merely derived on
// each call) - see IsOpen for why that matters.
const (
	stateClosed int32 = iota
	stateOpen
	stateHalfOpen
)

// Breaker is a single-endpoint circuit breaker: closed, open, half-open, with
// the half-open state gated to exactly one in-flight probe at a time. It
// holds no key and no map - callers own one Breaker per endpoint (or key it
// themselves, as adapter/health does with a single shared map).
type Breaker struct {
	failures     atomic.Int64
	lastFailure  atomic.Int64
	lastAttempt  atomic.Int64
	state        atomic.Int32
	threshold    int64
	openDuration time.Duration
}

// New builds a closed Breaker with the given configuration.
func New(cfg Config) *Breaker {
	return &Breaker{
		threshold:    int64(cfg.FailureThreshold),
		openDuration: cfg.OpenDuration,
	}
}

// NewTripped builds a Breaker already in the open state, with its last
// failure backdated by failedAgo before now. Exported for test setup that
// needs a breaker whose recovery timeout has already elapsed (eligible to
// transition to half-open) without sleeping in the test.
func NewTripped(cfg Config, failedAgo time.Duration) *Breaker {
	b := New(cfg)
	b.state.Store(stateOpen)
	b.lastFailure.Store(time.Now().Add(-failedAgo).UnixNano())
	return b
}

// IsOpen reports whether requests should be blocked. probeStaleness is the
// half-open single-flight staleness window - see Config for why it is a call
// parameter rather than fixed at construction.
//
// The second return value, attempt, identifies the specific half-open probe
// this call admitted (0 when the circuit is closed and no half-open gating
// applies, and 0 when this caller was rejected). Callers that proceed to
// perform the probe MUST pass attempt back into the matching
// RecordSuccess/RecordFailure call - see those for why: a late result from a
// probe that was superseded by a stale-handover replacement must not clobber
// the replacement's state.
func (b *Breaker) IsOpen(probeStaleness time.Duration) (open bool, attempt int64) {
	state := b.state.Load()
	if state == stateClosed {
		return false, 0
	}

	if state == stateOpen {
		lastFailure := b.lastFailure.Load()
		if !time.Unix(0, lastFailure).Add(b.openDuration).Before(time.Now()) {
			// Recovery timeout hasn't elapsed yet - stay fully open.
			return true, 0
		}
		// Timeout elapsed: attempt open -> half-open. Every caller that
		// reaches here (whether it won this CAS or another goroutine already
		// flipped it) falls through to the half-open single-flight gate
		// below. half-open is a persisted state, not merely "isOpen &&
		// timeout elapsed" recomputed on every call, mirroring olla's
		// original mechanics exactly (this is a pure refactor, not a
		// behaviour change). The practical effect - RecordFailure only
		// escalates back to stateOpen when failures >= threshold, so a
		// half-open failure below threshold would leave the breaker in
		// half-open rather than forcing a fresh openDuration wait - is not
		// reachable through the public API today: failures only resets on
		// success, so every non-closed state already has failures >=
		// threshold by construction, and RecordFailure always re-escalates.
		// The branch exists for future-proofing (e.g. a caller configuring a
		// threshold that legitimately allows several half-open attempts), not
		// because current callers depend on it.
		b.state.CompareAndSwap(stateOpen, stateHalfOpen)
