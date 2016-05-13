package eventbus

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestWorkerPool_NoGoroutineLeaks verifies the worker pool doesn't leak goroutines
func TestWorkerPool_NoGoroutineLeaks(t *testing.T) {
	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	// Create EventBus with worker pool
	eb := New[int]()

	// Subscribe to events
	ctx, cancel := context.WithCancel(context.Background())
	ch, cleanup := eb.Subscribe(ctx)
	defer cleanup()
	defer cancel()

	// Publish many events asynchronously
	const numEvents = 10000
	for i := range numEvents {
		eb.PublishAsync(i)
	}

	// Count received events
	received := 0
	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case <-ch:
			received++
			if received >= numEvents/2 { // Just check we got a good portion
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	// Shutdown EventBus. Shutdown() itself blocks on wg.Wait() until every
	// worker goroutine has exited, so the worker pool is deterministically
	// gone by the time this call returns - no polling needed for that part.
	eb.Shutdown()

	// What's left to settle asynchronously is the cleanup-ticker loop exiting
	// on close(stopCleanup) and (if still running) the per-subscriber
	// ctx-cancellation watcher goroutine - neither exposes a completion
	// signal, and goroutine-count-based leak detection has no synchronisation
	// seam by nature (it's runtime/scheduler state, not an event the
	// component emits). Both exit unconditionally given enough wall-clock, so
	// polling here is guaranteed eventually true by construction.
	var finalGoroutines, leaked int
	require.Eventually(t, func() bool {
		runtime.GC()
		finalGoroutines = runtime.NumGoroutine()
		leaked = finalGoroutines - baselineGoroutines
		return leaked <= 5 // small tolerance for test framework overhead
	}, pollCeiling, pollInterval, "goroutine leak: expected leaked <= 5")

	t.Logf("Baseline goroutines: %d", baselineGoroutines)
	t.Logf("Final goroutines: %d", finalGoroutines)
	t.Logf("Events published: %d", numEvents)
	t.Logf("Events received: %d", received)
	t.Logf("Leaked goroutines: %d", leaked)
}

// TestWorkerPool_HandlesBackpressure verifies the worker pool handles backpressure
func TestWorkerPool_HandlesBackpressure(t *testing.T) {
	// Create EventBus with small buffer
	config := EventBusConfig{
		BufferSize:    10,
		CleanupPeriod: 0, // Disable cleanup for this test
	}
	eb := NewWithConfig[int](config)

	// Create a slow subscriber
	ctx := context.Background()
	ch, _ := eb.Subscribe(ctx)
	// Don't use cleanup in this test - let Shutdown handle it
	defer eb.Shutdown()

	// Track dropped events
	var published atomic.Int64
	var received atomic.Int64

	// Slow consumer
	go func() {
		for range ch {
			received.Add(1)
			time.Sleep(time.Millisecond) // Simulate slow processing
		}
	}()

	// Publish many events rapidly, synchronising on the publisher goroutine's
	// own completion (a WaitGroup it owns) instead of polling a counter.
	var publishWg sync.WaitGroup
	publishWg.Add(1)
	go func() {
		defer publishWg.Done()
		for i := range 1000 {
			eb.PublishAsync(i)
			published.Add(1)
		}
	}()
	publishWg.Wait()

	// Drain blocks until every queued event has run through bus.Publish, so
	// the subscriber-buffer-full decision for every one of them is final by
	// the time it returns. With a 10-slot buffer against 1000 events and a
	// consumer that only drains one event per millisecond, overflow is
	// guaranteed by construction (workers dispatch far faster than the
	// consumer can keep the buffer clear), so the drop count check below
	// needs no polling.
	eb.Drain()

	require.Greater(t, eb.Stats().TotalDropped, uint64(0), "expected some events to be dropped due to backpressure")

	t.Logf("Published: %d", published.Load())
	t.Logf("Received: %d", received.Load())
}

// TestWorkerPool_PublishAsyncShutdownRace exercises the TOCTOU window that existed
// between PublishAsync's ctx check and its eventChan send when Shutdown closed the
// channel concurrently. The fix removes close(eventChan) from Shutdown; workers
// exit via ctx cancellation so the close was never necessary.
// Run with -race to verify no data race or send-on-closed-channel panic.
func TestWorkerPool_PublishAsyncShutdownRace(t *testing.T) {
	t.Parallel()

	const goroutines = 50
	const iterations = 200

	for trial := range 5 {
		_ = trial
		eb := New[int]()

		var wg sync.WaitGroup

		// Hammer PublishAsync from many goroutines.
		for g := range goroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := range iterations {
					eb.PublishAsync(id*iterations + i)
				}
			}(g)
		}

		// Shutdown concurrently with the senders. This used to race and could
		// panic on close(eventChan) being observed by a concurrent sender.
		eb.Shutdown()
		wg.Wait()
	}
}

// TestWorkerPool_QueueDroppedCountsOverflow verifies that events which never
// make it into the worker queue (because it's full) are counted separately
// from subscriber-buffer drops. Workers are set to zero so nothing ever
// drains eventChan, giving a deterministic fill point instead of racing a
// slow consumer against the publisher.
func TestWorkerPool_QueueDroppedCountsOverflow(t *testing.T) {
	eb := New[int]()
	defer eb.Shutdown()

	const bufferSize = 5
	wp := NewWorkerPool(eb, 0, bufferSize)
