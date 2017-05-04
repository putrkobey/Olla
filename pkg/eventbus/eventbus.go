package eventbus

/*
 * EventBus - A high-performance, lock-free pub/sub-system for Go
 * Copyright (c) 2016-2025 Thushan Fernando, Jason Wright and contributors
 *
 * Ported from Scout (2023) and updated to xsync v4 for better performance and safety (2025)
 */
import (
	"context"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
)

// EventBus provides lock-free pub/sub with automatic cleanup and backpressure handling
type EventBus[T any] struct {
	subscribers   *xsync.Map[string, *subscriber[T]]
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	workerPool    *WorkerPool[T]
	subscriberSeq atomic.Uint64
	bufferSize    int
	cleanupPeriod time.Duration
	isShutdown    atomic.Bool
}

type subscriber[T any] struct {
	ch         chan T
	id         string
	lastActive atomic.Int64
	dropped    atomic.Uint64
	isActive   atomic.Bool
}

// EventBusConfig allows customisation of buffer sizes and cleanup behaviour
type EventBusConfig struct {
	BufferSize      int
	CleanupPeriod   time.Duration
	InactiveTimeout time.Duration
}

var DefaultConfig = EventBusConfig{
	BufferSize:      100,
	CleanupPeriod:   5 * time.Minute,
	InactiveTimeout: 10 * time.Minute,
}

// New creates a new EventBus with default configuration
func New[T any]() *EventBus[T] {
	return NewWithConfig[T](DefaultConfig)
}
