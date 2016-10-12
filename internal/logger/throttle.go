package logger

import (
	"strings"
	"sync"
	"time"
)

const (
	// DefaultThrottleWindow is how long a key stays quiet after its first event
	// is logged in full.
	DefaultThrottleWindow = 10 * time.Second

	// DefaultThrottleMaxKeys caps tracked keys. A storm of distinct client IPs
	// (spoofed or genuine) then costs at most this many full lines per window,
	// plus one shared summary, instead of one line each.
	DefaultThrottleMaxKeys = 256

	// MaxThrottleKeyLen bounds a stored key. Behind trusted proxies the key can be the raw
	// first X-Forwarded-For value, which an attacker controls up to the header size; the
	// table holds MaxKeys of them and each is echoed in a summary line.
	MaxThrottleKeyLen = 64

	// OverflowKey labels the shared bucket that absorbs keys arriving once the
	// cap is reached.
	OverflowKey = "other"
)

// ThrottleConfig configures a Throttle. Zero values take the defaults.
type ThrottleConfig struct {
	// OnSummary receives the count of events suppressed for a key. It is called
	// outside the throttle's lock, so it may log freely. Nil drops summaries.
	OnSummary func(key string, suppressed int, window time.Duration)
	// Now is the clock; tests inject a fake. Nil means time.Now.
	Now     func() time.Time
	Window  time.Duration
	MaxKeys int
}

type throttleEntry struct {
	start      time.Time
	suppressed int
}

type throttleSummary struct {
	key        string
	suppressed int
}

// Throttle bounds how often an attacker-triggerable rejection reaches the log.
// The first event per key per window is allowed through (the caller logs it in
// full); the rest are counted, and the count is reported once through
// OnSummary when the window rolls over or the key is evicted. Metrics counters
// stay the source of truth: the throttle only decides what is written.
//
// Memory is O(MaxKeys) whatever the key cardinality: once the cap is reached,
// new keys share a single overflow bucket. It starts no goroutines, so there is
// nothing to leak; summaries for a flood that has stopped are written by the
// next event for the key, by the sweep that runs inside Allow once a window has
// lapsed, or by Flush. Allow is only called on a rejection, so a sweep needs another
// rejection through this same throttle: with none, nothing is written until the next
// event or Flush, which is why owners flush their throttles when they shut down.
type Throttle struct {
	lastSweep time.Time
	now       func() time.Time
	onSummary func(key string, suppressed int, window time.Duration)
	entries   map[string]*throttleEntry
	overflow  throttleEntry
	window    time.Duration
	maxKeys   int
	mu        sync.Mutex
