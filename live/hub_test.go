package live_test

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/auditlog-core/live"
)

func testEvent(seq int) json.RawMessage {
	return json.RawMessage(`{"sequence":` + strconv.Itoa(seq) + `,"event_type":"test"}`)
}

func TestHub_SubscribeUnsubscribe(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients, got %d", hub.ClientCount())
	}

	sub := hub.Subscribe()
	if sub == nil {
		t.Fatal("Subscribe returned nil")
	}

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Unsubscribe(sub.ID())

	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients after unsubscribe, got %d", hub.ClientCount())
	}
}

func TestHub_UnsubscribeNotFound(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()
	hub.Unsubscribe(999) // should not panic
}

func TestHub_OnEventDelivery(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()
	sub := hub.Subscribe()

	evt := testEvent(42)
	hub.OnEvent(evt)

	select {
	case got := <-sub.Events():
		var parsed struct {
			Sequence int `json:"sequence"`
		}
		if err := json.Unmarshal(got, &parsed); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}

		if parsed.Sequence != 42 {
			t.Fatalf("expected sequence 42, got %d", parsed.Sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHub_OnEventBroadcastsToMultipleSubscribers(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()
	sub1 := hub.Subscribe()
	sub2 := hub.Subscribe()
	sub3 := hub.Subscribe()

	evt := testEvent(1)
	hub.OnEvent(evt)

	for i, sub := range []*live.Subscriber{sub1, sub2, sub3} {
		select {
		case got := <-sub.Events():
			if len(got) == 0 {
				t.Fatalf("subscriber %d received empty event", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out", i)
		}
	}
}

func TestHub_SignalComplete(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	if hub.IsComplete() {
		t.Fatal("expected IsComplete to be false before signal")
	}

	sub := hub.Subscribe()

	hub.SignalComplete()

	if !hub.IsComplete() {
		t.Fatal("expected IsComplete to be true after signal")
	}

	select {
	case <-sub.Done():
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber Done channel")
	}
}

func TestHub_NonBlockingOnFullBuffer(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()
	sub := hub.Subscribe()

	// Fill the buffer (128) + 200 more. OnEvent must never block.
	for i := 0; i < 328; i++ {
		hub.OnEvent(testEvent(i))
	}

	// Drain what we can.
	received := 0
	draining := true
	for draining {
		select {
		case <-sub.Events():
			received++
		default:
			draining = false
		}
	}

	// Should have received exactly the buffer size (128).
	if received != 128 {
		t.Fatalf("expected 128 events from buffer, got %d", received)
	}
}

func TestHub_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()

			sub := hub.Subscribe()
			hub.OnEvent(testEvent(id))
			hub.IsComplete()
			hub.ClientCount()
			hub.Unsubscribe(sub.ID())
		}(i)
	}

	wg.Wait()

	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients after concurrent test, got %d", hub.ClientCount())
	}
}
