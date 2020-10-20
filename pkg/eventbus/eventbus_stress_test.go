package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// hangGuard bounds how long a storm test's publish+drain phase is allowed to
// take before we conclude it has wedged. It is a deadlock detector, not a
// success condition: reaching it fails the test with a clear "hung" message,
// but nothing here treats "finished within hangGuard" as proof of correct
// behaviour - the assertions after the guard do that.
const hangGuard = 60 * time.Second

// TestWorkerPool_ConcurrentPublishingStress hammers the bus with 10 publishers
// racing unthrottled against a single subscriber. PublishAsync is non-blocking
// by design, so under a genuine storm the bus WILL drop events at the queue
// and/or subscriber stage rather than block anyone - on a starved 2-core CI
// runner the receiver goroutine can lose enough scheduler time that most
// events are dropped before it ever gets to read them. There is no delivery
// floor to assert here; once the storm ends, dropped events are gone for
// good, not "eventually" delivered.
//
// This test proves two things deterministically: the storm doesn't wedge the
// bus (hangGuard), and once Drain confirms every queued event has been fully
// processed, delivered + tracked drops reconciles EXACTLY against what was
// published - not a lower bound, because Drain removes all ambiguity about
// events still mid-flight.
// This test is skipped in CI (when -short flag is used)
func TestWorkerPool_ConcurrentPublishingStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	eb := New[string]()

	ctx := context.Background()
	ch, cleanup := eb.Subscribe(ctx)
	defer cleanup()
	defer eb.Shutdown()

	// Track published vs received
	var published atomic.Int64
	var receivedCount atomic.Int64
	received := make(map[string]bool)
	var mu sync.Mutex

	// Original scale for stress testing
	const numPublishers = 10
	const eventsPerPublisher = 100

	// Start receiver
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case event := <-ch:
				receivedCount.Add(1)
				mu.Lock()
				received[event] = true
				mu.Unlock()
			case <-done:
				return
			}
		}
	}()
