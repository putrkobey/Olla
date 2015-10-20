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

	// Publish events rapidly (no delays - maximum stress)
	var wg sync.WaitGroup
	for p := range numPublishers {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()
			for i := range eventsPerPublisher {
				event := string(rune('A'+publisherID)) + string(rune('0'+i))
				eb.PublishAsync(event)
				published.Add(1)
			}
		}(p)
	}

	// Publish then Drain, guarded by hangGuard rather than polling for the
	// receive count to "stabilise". Drain blocks until every event that made
	// it into the worker queue has been run through bus.Publish, so once it
	// returns the subscriber-delivery decision for every published event is
	// final - a deterministic synchronisation point instead of a wall-clock
	// guess at when the storm has settled.
	stormDone := make(chan struct{})
	go func() {
		defer close(stormDone)
		wg.Wait()
		eb.Drain()
	}()

	select {
	case <-stormDone:
	case <-time.After(hangGuard):
		t.Fatal("test hung / suspected deadlock: storm publish+drain did not complete")
	}

	// Stop the receiver, then flush anything it hadn't yet pulled off the
	// channel: Drain has already returned, so nothing writes to ch anymore -
	// this sweep is a safe, deterministic drain rather than a race against
	// the receiver goroutine's own scheduling.
	close(done)
	<-stopped
flush:
	for {
		select {
		case event := <-ch:
			receivedCount.Add(1)
			mu.Lock()
			received[event] = true
			mu.Unlock()
		default:
			break flush
		}
	}

	publishedTotal := published.Load()
	receivedTotal := receivedCount.Load()

	mu.Lock()
	uniqueEvents := len(received)
	mu.Unlock()

	t.Logf("STRESS TEST - Published: %d", publishedTotal)
	t.Logf("STRESS TEST - Received: %d events", receivedTotal)
	t.Logf("STRESS TEST - Unique events: %d", uniqueEvents)

	// Prove the bus survived the storm rather than silently wedging: publish
	// one more event synchronously and confirm the still-live subscriber gets it.
	delivered := eb.Publish("post-storm")
	require.Equal(t, 1, delivered, "bus should still deliver to the live subscriber after the storm")

	// With every event drained and accounted for, delivered + tracked drops
	// (subscriber-stage and queue-stage) must reconcile EXACTLY against what
	// was published - there is no longer any event that could still be
