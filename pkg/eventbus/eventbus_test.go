package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// pollCeiling and pollInterval bound the require.Eventually polls across the
// eventbus package's tests: generous enough to absorb scheduler jitter on a
// loaded CI runner, tight enough to keep the suite fast on the happy path.
const (
	pollCeiling  = 5 * time.Second
	pollInterval = 20 * time.Millisecond
)

type TestEvent struct {
	ID      int
	Message string
}

type HealthEvent struct {
	EndpointName string
	Status       string
	Timestamp    time.Time
}

func TestEventBus_BasicPubSub(t *testing.T) {
	bus := New[TestEvent]()
	defer bus.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, cleanup := bus.Subscribe(ctx)
	defer cleanup()

	testEvent := TestEvent{ID: 1, Message: "test"}
	delivered := bus.Publish(testEvent)

	if delivered != 1 {
		t.Errorf("Expected 1 delivery, got %d", delivered)
	}

	select {
	case received := <-events:
		if received.ID != testEvent.ID || received.Message != testEvent.Message {
			t.Errorf("Event mismatch: expected %+v, got %+v", testEvent, received)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event")
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := New[TestEvent]()
	defer bus.Shutdown()

	ctx := context.Background()
	numSubscribers := 5
	var subscribers []<-chan TestEvent
	var cleanups []func()

	for range numSubscribers {
		events, cleanup := bus.Subscribe(ctx)
		subscribers = append(subscribers, events)
		cleanups = append(cleanups, cleanup)
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	testEvent := TestEvent{ID: 42, Message: "broadcast"}
	delivered := bus.Publish(testEvent)

	if delivered != numSubscribers {
		t.Errorf("Expected %d deliveries, got %d", numSubscribers, delivered)
	}

	for i, events := range subscribers {
		select {
		case received := <-events:
			if received.ID != testEvent.ID {
				t.Errorf("Subscriber %d: expected ID %d, got %d", i, testEvent.ID, received.ID)
			}
		case <-time.After(time.Second):
			t.Errorf("Subscriber %d: timeout waiting for event", i)
		}
	}
}

func TestEventBus_ContextCancellation(t *testing.T) {
	bus := New[TestEvent]()
	defer bus.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	events, cleanup := bus.Subscribe(ctx)
	defer cleanup()

	cancel()

	// Subscribe's cancellation watcher goroutine only fires eb.unsubscribe
	// after ctx.Done() closes, with no completion signal exposed to the
	// caller - polling Stats() is the only seam available, and the
	// unsubscribe itself is a single cheap map delete, not something
	// contending with backpressure, so it is guaranteed eventually true by
	// construction regardless of runner core count.
	require.Eventually(t, func() bool {
		return bus.Stats().TotalSubscribers == 0
	}, pollCeiling, pollInterval, "subscriber was not unsubscribed after context cancellation")

	// Verify no more events are received (channel not closed to prevent panics)
	select {
	case event := <-events:
		t.Errorf("Should not receive events after context cancellation, got: %+v", event)
	case <-time.After(100 * time.Millisecond):
		// Expected - no events after unsubscribe
	}
}

func TestEventBus_BackpressureHandling(t *testing.T) {
	config := EventBusConfig{
		BufferSize:      2,         // Small buffer to test backpressure
		CleanupPeriod:   time.Hour, // Disable cleanup for this test
		InactiveTimeout: time.Hour,
	}
	bus := NewWithConfig[TestEvent](config)
	defer bus.Shutdown()

	ctx := context.Background()
	events, cleanup := bus.Subscribe(ctx)
	defer cleanup()

	// Fill the buffer completely
	for i := range 2 {
		delivered := bus.Publish(TestEvent{ID: i, Message: "fill buffer"})
		if delivered != 1 {
			t.Errorf("Event %d: expected 1 delivery, got %d", i, delivered)
		}
	}

	// This should be dropped due to full buffer
	delivered := bus.Publish(TestEvent{ID: 999, Message: "should be dropped"})
	if delivered != 0 {
		t.Errorf("Expected 0 deliveries (buffer full), got %d", delivered)
	}

	stats := bus.Stats()
	if stats.TotalDropped == 0 {
		t.Error("Expected at least 1 dropped message")
	}

	// Drain buffer to verify first events made it through
	for i := range 2 {
		select {
		case event := <-events:
			if event.ID != i {
				t.Errorf("Expected event ID %d, got %d", i, event.ID)
			}
		case <-time.After(time.Second):
			t.Errorf("Timeout waiting for buffered event %d", i)
		}
	}
}
func TestEventBus_ConcurrentPublishSubscribe(t *testing.T) {
	bus := New[TestEvent]()
	defer bus.Shutdown()

	const numPublishers = 10
	const numSubscribers = 5
	const eventsPerPublisher = 100

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var subscribers []<-chan TestEvent
	var cleanups []func()
	receivedCounts := make([]int64, numSubscribers)
	var subscriberWg sync.WaitGroup

	for i := range numSubscribers {
		events, cleanup := bus.Subscribe(ctx)
		subscribers = append(subscribers, events)
		cleanups = append(cleanups, cleanup)

		idx := i
		subscriberWg.Add(1)
		go func() {
			defer subscriberWg.Done()
			for {
				select {
				case <-events:
					atomic.AddInt64(&receivedCounts[idx], 1)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	var wg sync.WaitGroup
	totalPublished := int64(0)

	for i := range numPublishers {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()
