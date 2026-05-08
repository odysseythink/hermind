package api

import (
	"sync"
)

// StreamEvent is a single streaming event published to all current SSE
// subscribers. The shape is intentionally open: the SSE handler
// JSON-serializes each event verbatim, and the frontend matches on Type.
type StreamEvent struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// StreamHub is the fan-out the SSE handler subscribes to. Publishers
// call Publish as they produce events; subscribers receive a channel of
// events until they call the unsubscribe function returned by Subscribe.
type StreamHub interface {
	Publish(ev StreamEvent)
	Subscribe() (<-chan StreamEvent, func())
}

// NewMemoryStreamHub returns an in-process StreamHub.
func NewMemoryStreamHub() *MemoryStreamHub {
	return &MemoryStreamHub{
		subs: make(map[subID]chan StreamEvent),
	}
}

type subID uint64

// MemoryStreamHub is the default in-process StreamHub. Single global
// fan-out — every subscriber gets every event.
type MemoryStreamHub struct {
	mu   sync.RWMutex
	next subID
	subs map[subID]chan StreamEvent
}

// Publish implements StreamHub.
func (h *MemoryStreamHub) Publish(ev StreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default:
			// Drop rather than block.
		}
	}
}

// Subscribe implements StreamHub.
func (h *MemoryStreamHub) Subscribe() (<-chan StreamEvent, func()) {
	ch := make(chan StreamEvent, 64)

	h.mu.Lock()
	h.next++
	id := h.next
	h.subs[id] = ch
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if c, ok := h.subs[id]; ok {
			close(c)
			delete(h.subs, id)
		}
	}
	return ch, unsubscribe
}
