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

