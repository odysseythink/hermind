package api

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStreamHub_PublishSubscribe(t *testing.T) {
	h := NewMemoryStreamHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, _ := h.Subscribe(ctx, "s1")
	h.Publish(StreamEvent{Type: "token", SessionID: "s1", Data: "hi"})

	select {
	case ev := <-ch:
		if ev.Type != "token" || ev.SessionID != "s1" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}
}

func TestMemoryStreamHub_IsolatedBySession(t *testing.T) {
	h := NewMemoryStreamHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chA, _ := h.Subscribe(ctx, "a")
	h.Publish(StreamEvent{SessionID: "b", Type: "token"})
	select {
	case ev := <-chA:
		t.Fatalf("session a got event for b: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMemoryStreamHub_UnsubscribeIdempotent(t *testing.T) {
	h := NewMemoryStreamHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, id := h.Subscribe(ctx, "s1")
	h.Unsubscribe("s1", id)
	h.Unsubscribe("s1", id) // must not panic.
}

func TestMemoryStreamHub_SlowSubscriberDoesNotBlock(t *testing.T) {
	h := NewMemoryStreamHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = h.Subscribe(ctx, "s1")
	// Publish more than the buffer size; the hub must drop rather than block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.Publish(StreamEvent{SessionID: "s1", Type: "token"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked on slow subscriber")
	}
}
