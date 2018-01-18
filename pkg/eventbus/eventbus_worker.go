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
