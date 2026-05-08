package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// Event is one item emitted by the bridge.
type Event struct {
	Cursor     int64  `json:"cursor"`
	Kind       string `json:"kind"` // "message" | "permission" | "session_update"
	SessionKey string `json:"session_key,omitempty"`
	Role       string `json:"role,omitempty"`
	Content    string `json:"content,omitempty"`
	At         int64  `json:"at,omitempty"`
}

// EventBridge buffers recent events and lets MCP clients poll or wait
// for them. It is driven by a Storage-backed poller that ticks on a
// fixed interval — tests can push events directly.
type EventBridge struct {
	store    storage.Storage
	interval time.Duration

	mu       sync.Mutex
	events   []Event
	capacity int

	subs map[chan Event]struct{}
}

// NewEventBridge constructs a bridge. Pass a nil Storage to get a
// poll-less bridge (useful for tests).
func NewEventBridge(store storage.Storage, interval time.Duration) *EventBridge {
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	return &EventBridge{
		store:    store,
		interval: interval,
		capacity: 1000,
		subs:     map[chan Event]struct{}{},
	}
}

// Run starts the background poll loop. Returns when ctx is cancelled.
// Safe to call with a nil Storage — the loop becomes a no-op.
func (b *EventBridge) Run(ctx context.Context) {
	if b.store == nil {
		<-ctx.Done()
		return
	}
	tick := time.NewTicker(b.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			b.pollOnce(ctx)
		}
	}
}

func (b *EventBridge) pollOnce(ctx context.Context) {
	msgs, err := b.store.GetHistory(ctx, 5, 0)
	if err != nil {
		return
	}
	for _, m := range msgs {
		b.push(Event{
			Cursor:     m.Timestamp.UnixMilli(),
			Kind:       "message",
			SessionKey: "instance",
			Role:       m.Role,
			Content:    truncate(m.Content, 400),
			At:         m.Timestamp.Unix(),
		})
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// push records an event and fans it out to subscribers. Safe for
// concurrent use.
func (b *EventBridge) push(ev Event) {
	b.mu.Lock()
	b.events = append(b.events, ev)
	if len(b.events) > b.capacity {
		b.events = b.events[len(b.events)-b.capacity:]
	}
	subs := make([]chan Event, 0, len(b.subs))
	for s := range b.subs {
		subs = append(subs, s)
	}
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// drop: subscriber is slow; they'll recover via Poll.
		}
	}
}

// Poll returns events with cursor > afterCursor (up to limit),
// filtered by session key if non-empty. The second return value is
// the new next-cursor to feed back in.
func (b *EventBridge) Poll(afterCursor int64, sessionKey string, limit int) ([]Event, int64) {
	if limit <= 0 {
		limit = 20
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, 0, limit)
	var last int64 = afterCursor
	for _, ev := range b.events {
		if ev.Cursor <= afterCursor {
			continue
		}
		if sessionKey != "" && ev.SessionKey != sessionKey {
			continue
		}
		out = append(out, ev)
		if ev.Cursor > last {
			last = ev.Cursor
		}
		if len(out) >= limit {
			break
		}
	}
	return out, last
}

// Wait blocks until an event matching the filter arrives or the
// timeout fires. Returns (nil, nil) on timeout.
func (b *EventBridge) Wait(ctx context.Context, afterCursor int64, sessionKey string, timeout time.Duration) (*Event, error) {
	if evs, _ := b.Poll(afterCursor, sessionKey, 1); len(evs) > 0 {
		ev := evs[0]
		return &ev, nil
	}
	ch := make(chan Event, 4)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if ev.Cursor <= afterCursor {
				continue
			}
			if sessionKey != "" && ev.SessionKey != sessionKey {
				continue
			}
			return &ev, nil
		case <-deadline:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ---- PermissionQueue ----

// PermissionRequest describes a pending human-approval request.
type PermissionRequest struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	Kind    string `json:"kind"`
	OpenAt  int64  `json:"open_at"`
}

// PermissionQueue is the in-memory set of open permission requests.
type PermissionQueue struct {
	mu       sync.Mutex
	open     map[string]PermissionRequest
	outcomes map[string]chan string
}

// NewPermissionQueue constructs a queue.
func NewPermissionQueue() *PermissionQueue {
	return &PermissionQueue{
		open:     map[string]PermissionRequest{},
		outcomes: map[string]chan string{},
	}
}

// Open registers a new request and returns its ID.
func (q *PermissionQueue) Open(req PermissionRequest) string {
	id := req.ID
	if id == "" {
		var buf [6]byte
		_, _ = rand.Read(buf[:])
		id = "perm-" + hex.EncodeToString(buf[:])
	}
	req.ID = id
	req.OpenAt = time.Now().Unix()
	q.mu.Lock()
	q.open[id] = req
	q.outcomes[id] = make(chan string, 1)
	q.mu.Unlock()
	return id
}

// ListOpen returns all pending requests.
func (q *PermissionQueue) ListOpen() []PermissionRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]PermissionRequest, 0, len(q.open))
	for _, r := range q.open {
		out = append(out, r)
	}
	return out
}

// Respond records a decision. Returns false if the ID is not open.
func (q *PermissionQueue) Respond(id, decision string) bool {
	q.mu.Lock()
	req, ok := q.open[id]
	outCh := q.outcomes[id]
	if ok {
		delete(q.open, id)
		delete(q.outcomes, id)
	}
	q.mu.Unlock()
	if !ok {
		return false
	}
	_ = req
	select {
	case outCh <- decision:
	default:
	}
	return true
}

// Await blocks for the decision on this request. Useful for wiring
// into tool execution paths.
func (q *PermissionQueue) Await(id string, timeout time.Duration) (string, bool) {
	q.mu.Lock()
	ch := q.outcomes[id]
	q.mu.Unlock()
	if ch == nil {
		return "", false
	}
	select {
	case dec := <-ch:
		return dec, true
	case <-time.After(timeout):
		return "", false
	}
}
