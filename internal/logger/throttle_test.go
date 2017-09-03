package logger

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	t  time.Time
	mu sync.Mutex
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type summaryRecorder struct {
	got map[string]int
	mu  sync.Mutex
	n   int
}

func (r *summaryRecorder) record(key string, suppressed int, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.got == nil {
		r.got = map[string]int{}
	}
	r.got[key] += suppressed
	r.n++
}

func (r *summaryRecorder) total() (calls, suppressed int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.got {
		suppressed += v
	}
	return r.n, suppressed
}

func TestThrottle_FirstEventLoggedRestSuppressedWithExactSummary(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	rec := &summaryRecorder{}
	th := NewThrottle(ThrottleConfig{Window: 10 * time.Second, Now: clock.Now, OnSummary: rec.record})

	allowed := 0
	for range 1000 {
		if th.Allow("1.2.3.4") {
			allowed++
		}
	}
	require.Equal(t, 1, allowed)
	calls, _ := rec.total()
	require.Zero(t, calls, "summary waits for the window to end")

	// window rollover: next event is logged in full and the previous window's
	// 999 suppressed events are reported exactly once
	clock.Advance(10 * time.Second)
	require.True(t, th.Allow("1.2.3.4"))
	calls, suppressed := rec.total()
	require.Equal(t, 1, calls)
	require.Equal(t, 999, suppressed)
	require.Equal(t, 999, rec.got["1.2.3.4"])

	// a quiet window produces no empty summary
	clock.Advance(10 * time.Second)
	require.True(t, th.Allow("1.2.3.4"))
	calls, _ = rec.total()
	require.Equal(t, 1, calls)
}

func TestThrottle_KeysAreIndependent(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	th := NewThrottle(ThrottleConfig{Now: clock.Now})

	require.True(t, th.Allow("a"))
	require.True(t, th.Allow("b"))
	require.False(t, th.Allow("a"))
	require.False(t, th.Allow("b"))
}

func TestThrottle_FlushReportsTailOnce(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	rec := &summaryRecorder{}
	th := NewThrottle(ThrottleConfig{Now: clock.Now, OnSummary: rec.record})

	for range 11 {
		th.Allow("a")
	}
	th.Flush()
	th.Flush()
	calls, suppressed := rec.total()
	require.Equal(t, 1, calls)
	require.Equal(t, 10, suppressed)
}

func TestThrottle_CapsKeysAndOverflowSharesOneBucket(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	rec := &summaryRecorder{}
	const maxKeys = 8
	th := NewThrottle(ThrottleConfig{MaxKeys: maxKeys, Now: clock.Now, OnSummary: rec.record})

	full := 0
	for i := range 100 {
		if th.Allow(fmt.Sprintf("ip-%d", i)) {
			full++
		}
	}
	// 8 tracked keys plus the first event into the shared overflow bucket
	require.Equal(t, maxKeys+1, full)
	require.Equal(t, maxKeys, th.Len())

	// once the window lapses, idle slots are reclaimed and the overflow tail reported
	clock.Advance(DefaultThrottleWindow)
	require.True(t, th.Allow("new-ip"))
	_, suppressed := rec.total()
	require.Equal(t, 100-maxKeys-1, suppressed)
	require.Equal(t, 100-maxKeys-1, rec.got[OverflowKey])
	require.LessOrEqual(t, th.Len(), maxKeys)
}

// A spoofed-IP storm must neither grow memory nor flood the log. Run with
// -race: the shards of work share one throttle.
func TestThrottle_DistinctKeyStormStaysBounded(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	rec := &summaryRecorder{}
	th := NewThrottle(ThrottleConfig{Now: clock.Now, OnSummary: rec.record})

	const (
		total   = 100_000
		workers = 8
	)
