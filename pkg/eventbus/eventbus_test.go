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
