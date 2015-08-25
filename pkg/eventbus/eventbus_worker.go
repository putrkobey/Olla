package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
)

// WorkerPool manages a pool of workers for async event publishing
type WorkerPool[T any] struct {
	ctx       context.Context
	eventChan chan T
	bus       *EventBus[T]
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	// pending tracks events that have been successfully queued but not yet
	// run through bus.Publish, so Drain can block until the queue is
	// genuinely idle rather than just channel-empty (see PublishAsync/worker).
	pending sync.WaitGroup
	workers int
	dropped atomic.Uint64
}

// NewWorkerPool creates a new worker pool for the event bus
func NewWorkerPool[T any](bus *EventBus[T], workers int, bufferSize int) *WorkerPool[T] {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool[T]{
		eventChan: make(chan T, bufferSize),
		bus:       bus,
		workers:   workers,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start worker goroutines
	for range workers {
		wp.wg.Add(1)
		go wp.worker()
	}

	return wp
}

// PublishAsync queues an event for async publishing
func (wp *WorkerPool[T]) PublishAsync(event T) {
	// Check if we're shutting down
	select {
	case <-wp.ctx.Done():
		// Worker pool is shutting down, drop the event
		return
	default:
	}

	// Add before attempting the send, not after: a buffered channel send
	// returns as soon as there's room, with no guarantee the worker hasn't
	// already dequeued and processed the item before this goroutine gets to
	// its next statement. Adding afterwards let a worker's Done() race ahead
	// of the matching Add(), driving the counter negative under load.
	wp.pending.Add(1)

	// Try non-blocking send
	select {
	case wp.eventChan <- event:
		// Successfully queued - a worker will call Done() once it has run
		// this event through bus.Publish.
	default:
		// Queue is full, drop the event to prevent blocking. This is a
		// stage-1 drop - the event never reached a subscriber at all - which
		// is a different failure mode to EventBusStats.TotalDropped (a
		// stage-3 drop, where a subscriber's own buffer was full). Operators
		// need both counters to tell "queue too small / workers too slow to
		// drain" apart from "a subscriber is too slow to consume".
		wp.dropped.Add(1)
		wp.pending.Done() // compensate: this event will never reach a worker
	}
}

// Dropped returns the number of events discarded because the worker queue
// was full when PublishAsync was called.
func (wp *WorkerPool[T]) Dropped() uint64 {
	return wp.dropped.Load()
}

// worker processes events from the queue
func (wp *WorkerPool[T]) worker() {
	defer wp.wg.Done()
	for {
		select {
		case event, ok := <-wp.eventChan:
			if !ok {
				return // channel closed
			}
			wp.bus.Publish(event)
			wp.pending.Done()
		case <-wp.ctx.Done():
			return
		}
	}
}

// Drain blocks until every event queued via PublishAsync before this call
