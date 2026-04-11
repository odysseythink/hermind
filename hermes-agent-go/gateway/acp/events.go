package acp

import (
	"sync"
	"time"
)

// Event is one ACP server-sent event.
type Event struct {
	Type      string    `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	Data      string    `json:"data,omitempty"`
	Time      time.Time `json:"time"`
}

// EventBus is a tiny pub/sub used by the server to push events to
// one or more SSE subscribers. Every subscriber has its own buffered
// channel; slow subscribers drop old events rather than blocking the
// publisher.
type EventBus struct {
	mu          sync.Mutex
	subscribers []chan Event
}

func NewEventBus() *EventBus { return &EventBus{} }

// Subscribe returns a buffered channel that will receive every
// subsequent published event. Call Unsubscribe to release it.
func (b *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a previously returned channel.
func (b *EventBus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.subscribers[:0]
	for _, s := range b.subscribers {
		if s == ch {
			close(s)
			continue
		}
		out = append(out, s)
	}
	b.subscribers = out
}

// Publish broadcasts an event to every subscriber. Non-blocking:
// events are dropped for subscribers whose channel is full.
func (b *EventBus) Publish(ev Event) {
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	b.mu.Lock()
	subs := append([]chan Event{}, b.subscribers...)
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// drop
		}
	}
}
