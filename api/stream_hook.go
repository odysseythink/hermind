package api

import (
	"context"
	"sync"
)

// StreamEvent is a single streaming event published to subscribers of a
// session (token chunk, tool call, status change). The shape is
// intentionally open so the parallel WebSocket/SSE agent can adopt it
// without requiring changes to the REST surface.
type StreamEvent struct {
	// Type categorizes the event. Examples: "token", "tool_call",
	// "tool_result", "message_complete", "status".
	Type string `json:"type"`

	// SessionID identifies the session the event belongs to.
	SessionID string `json:"session_id"`

	// Data is an opaque payload specific to Type. Must be
	// JSON-serializable so the WebSocket / SSE layer can forward it
	// verbatim.
	Data any `json:"data,omitempty"`
}

// StreamHub is the hook the WebSocket / SSE agent attaches to. It is a
// tiny per-session fan-out that the REST server exposes via
// Server.Streams(). The agent loop calls Publish as it produces events;
// subscribers receive a channel of events until they unsubscribe or the
// context is cancelled.
//
// The interface is deliberately channel-based rather than callback-based
// so the WebSocket agent can use select{} with the connection's read
// loop. A callback-flavor helper (Subscribe + goroutine) can be layered
// on top without touching this package.
type StreamHub interface {
	// Publish broadcasts an event to every current subscriber of
	// ev.SessionID. It never blocks: a slow subscriber loses events
	// rather than stalling the publisher.
	Publish(ev StreamEvent)

	// Subscribe registers interest in a given session. The returned
	// channel receives events until ctx is cancelled or Unsubscribe is
	// called with the returned id. The buffer size is an
	// implementation detail but is never zero.
	Subscribe(ctx context.Context, sessionID string) (<-chan StreamEvent, SubscriptionID)

	// Unsubscribe releases a subscription. Safe to call more than once.
	Unsubscribe(sessionID string, id SubscriptionID)
}

// SubscriptionID identifies an open Subscribe call.
type SubscriptionID uint64

// NewMemoryStreamHub returns the default in-process StreamHub. It is
// safe for concurrent use. The WebSocket agent may swap this for a
// redis- or nats-backed implementation later by assigning a different
// StreamHub to ServerOpts.Streams.
func NewMemoryStreamHub() *MemoryStreamHub {
	return &MemoryStreamHub{
		subs: make(map[string]map[SubscriptionID]chan StreamEvent),
	}
}

// MemoryStreamHub is an in-process StreamHub backed by a map of
// per-session subscriber channels.
type MemoryStreamHub struct {
	mu   sync.RWMutex
	next SubscriptionID
	subs map[string]map[SubscriptionID]chan StreamEvent
}

// Publish implements StreamHub.
func (h *MemoryStreamHub) Publish(ev StreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs[ev.SessionID] {
		select {
		case ch <- ev:
		default:
			// Drop rather than block — slow subscriber loses events.
		}
	}
}

// Subscribe implements StreamHub.
func (h *MemoryStreamHub) Subscribe(ctx context.Context, sessionID string) (<-chan StreamEvent, SubscriptionID) {
	ch := make(chan StreamEvent, 64)

	h.mu.Lock()
	h.next++
	id := h.next
	if h.subs[sessionID] == nil {
		h.subs[sessionID] = make(map[SubscriptionID]chan StreamEvent)
	}
	h.subs[sessionID][id] = ch
	h.mu.Unlock()

	// Auto-cleanup when ctx cancels.
	go func() {
		<-ctx.Done()
		h.Unsubscribe(sessionID, id)
	}()

	return ch, id
}

// Unsubscribe implements StreamHub.
func (h *MemoryStreamHub) Unsubscribe(sessionID string, id SubscriptionID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.subs[sessionID]; ok {
		if ch, ok := m[id]; ok {
			close(ch)
			delete(m, id)
		}
		if len(m) == 0 {
			delete(h.subs, sessionID)
		}
	}
}
