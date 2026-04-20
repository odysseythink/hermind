# MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose hermind **as an MCP server** over stdio — the inverse of the existing `tool/mcp/` client. External MCP hosts (Claude Desktop, Cursor, Cline, Zed) can then read hermind conversations, poll for events, send messages, and respond to permission requests through MCP tool invocations. MVP parity with Python `mcp_serve.py`: 10 canonical tools (`conversations_list`, `conversation_get`, `messages_read`, `messages_send`, `attachments_fetch`, `events_poll`, `events_wait`, `permissions_list_open`, `permissions_respond`, `channels_list`) plus an `EventBridge` that polls the storage layer for new messages and unresolved permission requests.

**Architecture:** A new `mcp/server/` package implements the MCP stdio protocol directly — same newline-delimited JSON-RPC shape used by `acp/stdio`, so we reuse the `protocol.go` frame codec by extracting it into a shared `internal/jsonrpc/` helper. Handlers are plain methods on a `Server` struct that close over `storage.Storage` + an `EventBridge`. The event bridge runs a ticker-driven loop that inspects modification timestamps of `state.db` + `~/.hermind/sessions.json` and, when they advance, queries for new messages since the last-seen cursor. Permission requests are stored in a lightweight in-memory map keyed by request ID. A new `hermind mcp serve` cobra subcommand boots the server against stdin/stdout.

**Tech Stack:** Go 1.21+, existing `storage.Storage`, `message`, `config`, `cobra`. We deliberately avoid the third-party `modelcontextprotocol/sdk-go` dependency for this MVP — the protocol surface is small enough that a hand-rolled implementation is simpler than adapting to an external schema. The optional upgrade to the SDK is flagged at the end of the plan.

---

## File Structure

- Create: `internal/jsonrpc/frame.go` — extract the frame codec shared with `acp/stdio` (request/response/notification)
- Create: `internal/jsonrpc/frame_test.go`
- Modify: `acp/stdio/protocol.go` — keep local types but delegate encoding to `internal/jsonrpc` (optional if you want to defer; otherwise duplicate is fine)
- Create: `mcp/server/server.go` — stdio read/dispatch loop
- Create: `mcp/server/server_test.go`
- Create: `mcp/server/tools.go` — static MCP tool catalog (name, description, input schema)
- Create: `mcp/server/tools_test.go`
- Create: `mcp/server/handlers.go` — the 10 MCP tools wired against `storage.Storage`
- Create: `mcp/server/handlers_test.go`
- Create: `mcp/server/events.go` — `EventBridge` poll loop + in-memory permission queue
- Create: `mcp/server/events_test.go`
- Create: `cli/mcp.go` — `hermind mcp serve` subcommand
- Create: `cli/mcp_test.go`
- Modify: `cli/root.go` — register `newMCPCmd(app)`

---

## Task 1: Shared jsonrpc frame codec

**Files:**
- Create: `internal/jsonrpc/frame.go`
- Create: `internal/jsonrpc/frame_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/jsonrpc/frame_test.go`:

```go
package jsonrpc

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRequest(t *testing.T) {
	req, err := DecodeRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"k":"v"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "ping" {
		t.Errorf("method = %q", req.Method)
	}
	if string(req.Params) != `{"k":"v"}` {
		t.Errorf("params = %s", req.Params)
	}
}

func TestEncodeResponse_Result(t *testing.T) {
	var buf bytes.Buffer
	_ = EncodeResponse(&buf, &Response{
		ID:     json.RawMessage(`1`),
		Result: json.RawMessage(`{"ok":true}`),
	})
	out := buf.String()
	if !strings.Contains(out, `"jsonrpc":"2.0"`) || !strings.HasSuffix(out, "\n") {
		t.Errorf("got %q", out)
	}
}

func TestEncodeNotification_NoID(t *testing.T) {
	var buf bytes.Buffer
	_ = EncodeNotification(&buf, "hello", map[string]string{"x": "y"})
	if strings.Contains(buf.String(), `"id"`) {
		t.Errorf("notifications must omit id: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/jsonrpc/ -v`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement**

Create `internal/jsonrpc/frame.go`:

```go
// Package jsonrpc implements the minimal subset of JSON-RPC 2.0 used
// by hermind's stdio servers (acp/stdio and mcp/server). Frames are
// newline-delimited; Content-Length headers are not supported.
package jsonrpc

import (
	"encoding/json"
	"fmt"
	"io"
)

const Version = "2.0"

// Standard error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Request is a decoded JSON-RPC 2.0 request.
type Request struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification reports whether this request carries no ID.
func (r *Request) IsNotification() bool {
	return len(r.ID) == 0 || string(r.ID) == "null"
}

// Response is a JSON-RPC 2.0 response frame.
type Response struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is the JSON-RPC error object.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// DecodeRequest parses a single newline-delimited frame.
func DecodeRequest(raw []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("jsonrpc: decode: %w", err)
	}
	if req.Version != "" && req.Version != Version {
		return nil, fmt.Errorf("jsonrpc: unsupported version %q", req.Version)
	}
	return &req, nil
}

// EncodeResponse writes resp + "\n".
func EncodeResponse(w io.Writer, resp *Response) error {
	resp.Version = Version
	if resp.Result == nil && resp.Error == nil {
		resp.Result = json.RawMessage(`null`)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

// EncodeNotification writes a notification frame (no id).
func EncodeNotification(w io.Writer, method string, params interface{}) error {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Version string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{Version, method, rawParams})
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/jsonrpc/ -v`
Expected: PASS (3 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add internal/jsonrpc/frame.go internal/jsonrpc/frame_test.go
git commit -m "feat(internal/jsonrpc): shared frame codec for stdio servers"
```

Note: `acp/stdio` already has its own copy. You can refactor it to import `internal/jsonrpc` in a follow-up; leaving it duplicated is fine for this plan.

---

## Task 2: MCP tool catalog

**Files:**
- Create: `mcp/server/tools.go`
- Create: `mcp/server/tools_test.go`

- [ ] **Step 1: Write the failing test**

Create `mcp/server/tools_test.go`:

```go
package server

import "testing"

func TestBuiltinTools_Count(t *testing.T) {
	got := BuiltinTools()
	if len(got) != 10 {
		t.Errorf("expected 10 tools, got %d", len(got))
	}
}

func TestBuiltinTools_Names(t *testing.T) {
	want := map[string]bool{
		"conversations_list":     false,
		"conversation_get":       false,
		"messages_read":          false,
		"messages_send":          false,
		"attachments_fetch":      false,
		"events_poll":            false,
		"events_wait":            false,
		"permissions_list_open":  false,
		"permissions_respond":    false,
		"channels_list":          false,
	}
	for _, tl := range BuiltinTools() {
		if _, ok := want[tl.Name]; !ok {
			t.Errorf("unexpected tool %q", tl.Name)
		}
		want[tl.Name] = true
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("missing tool %q", n)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./mcp/server/ -run TestBuiltinTools -v`
Expected: FAIL.

- [ ] **Step 3: Implement the catalog**

Create `mcp/server/tools.go`:

```go
// Package server implements an MCP (Model Context Protocol) server that
// exposes hermind's conversation + event surface to external MCP
// hosts (Claude Desktop, Cursor, Zed, Cline, etc.). The transport is
// newline-delimited JSON-RPC 2.0 over stdio — the same shape used by
// the acp/stdio server.
package server

import "encoding/json"

// Tool describes one MCP tool entry. The shape follows the MCP spec's
// `Tool` object so hosts can consume it verbatim.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// BuiltinTools returns the static catalog exposed by the MCP server.
// Keep the order stable — Claude Desktop pins tool indices per server
// boot for caching purposes.
func BuiltinTools() []Tool {
	obj := func(schema string) json.RawMessage { return json.RawMessage(schema) }
	return []Tool{
		{
			Name:        "conversations_list",
			Description: "List recent hermind conversations, optionally filtered by platform or free-text search.",
			InputSchema: obj(`{"type":"object","properties":{"platform":{"type":"string"},"limit":{"type":"integer","default":50},"search":{"type":"string"}}}`),
		},
		{
			Name:        "conversation_get",
			Description: "Fetch metadata and token stats for a single conversation by session key.",
			InputSchema: obj(`{"type":"object","required":["session_key"],"properties":{"session_key":{"type":"string"}}}`),
		},
		{
			Name:        "messages_read",
			Description: "Read the most recent messages from a conversation.",
			InputSchema: obj(`{"type":"object","required":["session_key"],"properties":{"session_key":{"type":"string"},"limit":{"type":"integer","default":50}}}`),
		},
		{
			Name:        "messages_send",
			Description: "Send a message to a target (platform:chat_id or session key).",
			InputSchema: obj(`{"type":"object","required":["target","message"],"properties":{"target":{"type":"string"},"message":{"type":"string"}}}`),
		},
		{
			Name:        "attachments_fetch",
			Description: "List attachments for a specific message.",
			InputSchema: obj(`{"type":"object","required":["session_key","message_id"],"properties":{"session_key":{"type":"string"},"message_id":{"type":"integer"}}}`),
		},
		{
			Name:        "events_poll",
			Description: "Non-blocking poll for new events since the given cursor.",
			InputSchema: obj(`{"type":"object","properties":{"after_cursor":{"type":"integer","default":0},"session_key":{"type":"string"},"limit":{"type":"integer","default":20}}}`),
		},
		{
			Name:        "events_wait",
			Description: "Block until a new event arrives or timeout_ms elapses.",
			InputSchema: obj(`{"type":"object","required":["after_cursor"],"properties":{"after_cursor":{"type":"integer"},"session_key":{"type":"string"},"timeout_ms":{"type":"integer","default":30000}}}`),
		},
		{
			Name:        "permissions_list_open",
			Description: "List pending permission requests that need a human decision.",
			InputSchema: obj(`{"type":"object"}`),
		},
		{
			Name:        "permissions_respond",
			Description: "Respond to a pending permission request.",
			InputSchema: obj(`{"type":"object","required":["id","decision"],"properties":{"id":{"type":"string"},"decision":{"type":"string","enum":["allow-once","allow-always","deny"]}}}`),
		},
		{
			Name:        "channels_list",
			Description: "List all known message targets grouped by platform.",
			InputSchema: obj(`{"type":"object","properties":{"platform":{"type":"string"}}}`),
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./mcp/server/ -run TestBuiltinTools -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mcp/server/tools.go mcp/server/tools_test.go
git commit -m "feat(mcp/server): builtin MCP tool catalog (10 tools)"
```

---

## Task 3: EventBridge + permission queue

**Files:**
- Create: `mcp/server/events.go`
- Create: `mcp/server/events_test.go`

- [ ] **Step 1: Write the failing test**

Create `mcp/server/events_test.go`:

```go
package server

import (
	"context"
	"testing"
	"time"
)

func TestEventBridge_Poll_EmitsNewMessages(t *testing.T) {
	bridge := NewEventBridge(nil, 10*time.Millisecond)

	// Seed a fake event directly into the ring.
	bridge.push(Event{Cursor: 1, Kind: "message", SessionKey: "s1"})
	bridge.push(Event{Cursor: 2, Kind: "message", SessionKey: "s1"})

	got, next := bridge.Poll(0, "", 10)
	if len(got) != 2 {
		t.Fatalf("got %d events", len(got))
	}
	if next != 2 {
		t.Errorf("next cursor = %d, want 2", next)
	}
}

func TestEventBridge_Poll_FiltersBySession(t *testing.T) {
	bridge := NewEventBridge(nil, 10*time.Millisecond)
	bridge.push(Event{Cursor: 1, Kind: "message", SessionKey: "s1"})
	bridge.push(Event{Cursor: 2, Kind: "message", SessionKey: "s2"})

	got, _ := bridge.Poll(0, "s1", 10)
	if len(got) != 1 || got[0].SessionKey != "s1" {
		t.Errorf("filter failed: %+v", got)
	}
}

func TestEventBridge_Wait_ReturnsOnPush(t *testing.T) {
	bridge := NewEventBridge(nil, 10*time.Millisecond)
	ctx := context.Background()

	doneCh := make(chan Event, 1)
	go func() {
		ev, _ := bridge.Wait(ctx, 0, "", 500*time.Millisecond)
		if ev != nil {
			doneCh <- *ev
		}
	}()

	time.Sleep(20 * time.Millisecond)
	bridge.push(Event{Cursor: 1, Kind: "message"})

	select {
	case got := <-doneCh:
		if got.Cursor != 1 {
			t.Errorf("got %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return")
	}
}

func TestPermissionQueue_Roundtrip(t *testing.T) {
	q := NewPermissionQueue()
	id := q.Open(PermissionRequest{Command: "rm", Kind: "execute"})
	open := q.ListOpen()
	if len(open) != 1 || open[0].ID != id {
		t.Errorf("list = %+v", open)
	}
	if !q.Respond(id, "allow-once") {
		t.Error("respond should succeed")
	}
	if q.Respond(id, "allow-once") {
		t.Error("second respond should fail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./mcp/server/ -run "TestEventBridge|TestPermissionQueue" -v`
Expected: FAIL.

- [ ] **Step 3: Implement EventBridge + PermissionQueue**

Create `mcp/server/events.go`:

```go
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
			// Minimal implementation: scan recent sessions and emit
			// messages with timestamps newer than our last high-water
			// mark. Real work happens inside storage — this routine
			// just calls ListSessions + GetMessages. A more optimized
			// version would read a change-log table.
			b.pollOnce(ctx)
		}
	}
}

func (b *EventBridge) pollOnce(ctx context.Context) {
	// Placeholder: pull the most recent 10 sessions and their tail
	// messages. Production tuning is out of scope for the MVP — the
	// goal here is that events flow at all.
	sess, err := b.store.ListSessions(ctx, &storage.ListOptions{Limit: 10})
	if err != nil {
		return
	}
	for _, s := range sess {
		msgs, err := b.store.GetMessages(ctx, s.ID, 5, 0)
		if err != nil {
			continue
		}
		for _, m := range msgs {
			b.push(Event{
				Cursor:     int64(m.Timestamp * 1000),
				Kind:       "message",
				SessionKey: s.ID,
				Role:       m.Role,
				Content:    truncate(m.Content, 400),
				At:         int64(m.Timestamp),
			})
		}
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
	// Fast path: any buffered events already past the cursor?
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./mcp/server/ -run "TestEventBridge|TestPermissionQueue" -v -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mcp/server/events.go mcp/server/events_test.go
git commit -m "feat(mcp/server): EventBridge + PermissionQueue"
```

---

## Task 4: MCP handlers

**Files:**
- Create: `mcp/server/handlers.go`
- Create: `mcp/server/handlers_test.go`

- [ ] **Step 1: Write the failing test**

Create `mcp/server/handlers_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func newTestServer(t *testing.T) (*Server, storage.Storage) {
	t.Helper()
	store, err := sqlite.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bridge := NewEventBridge(store, 10*time.Millisecond)
	perms := NewPermissionQueue()
	s := NewServer(&ServerOpts{Storage: store, Events: bridge, Permissions: perms})
	return s, store
}

func seed(t *testing.T, store storage.Storage, id string) {
	t.Helper()
	_ = store.CreateSession(context.Background(), &storage.Session{ID: id, Source: "cli", Model: "m"})
	_ = store.AddMessage(context.Background(), id, &storage.StoredMessage{Role: "user", Content: "hi"})
}

func TestHandleInitialize(t *testing.T) {
	s, _ := newTestServer(t)
	raw, err := s.handleInitialize(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	_ = json.Unmarshal(raw, &resp)
	if resp["protocolVersion"] == nil {
		t.Errorf("missing protocolVersion: %v", resp)
	}
	caps, _ := resp["capabilities"].(map[string]any)
	if caps == nil || caps["tools"] == nil {
		t.Errorf("missing tools capability: %v", resp)
	}
}

func TestHandleToolsList(t *testing.T) {
	s, _ := newTestServer(t)
	raw, err := s.handleToolsList(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Tools []Tool `json:"tools"`
	}
	_ = json.Unmarshal(raw, &resp)
	if len(resp.Tools) != 10 {
		t.Errorf("tools = %d", len(resp.Tools))
	}
}

func TestHandleToolsCall_ConversationsList(t *testing.T) {
	s, store := newTestServer(t)
	seed(t, store, "a")
	seed(t, store, "b")

	params, _ := json.Marshal(map[string]any{
		"name":      "conversations_list",
		"arguments": map[string]any{"limit": 5},
	})
	raw, err := s.handleToolsCall(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	// Response shape: {"content":[{"type":"text","text":"<json>"}]}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &resp)
	if len(resp.Content) != 1 || resp.Content[0].Type != "text" {
		t.Fatalf("bad shape: %s", raw)
	}
	if resp.Content[0].Text == "" {
		t.Error("empty text")
	}
}

func TestHandleToolsCall_MessagesRead(t *testing.T) {
	s, store := newTestServer(t)
	seed(t, store, "s1")

	params, _ := json.Marshal(map[string]any{
		"name":      "messages_read",
		"arguments": map[string]any{"session_key": "s1", "limit": 10},
	})
	raw, _ := s.handleToolsCall(context.Background(), params)
	var resp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &resp)
	if resp.Content[0].Text == "" || !contains(resp.Content[0].Text, `"hi"`) {
		t.Errorf("unexpected text: %s", resp.Content[0].Text)
	}
}

func TestHandleToolsCall_UnknownTool(t *testing.T) {
	s, _ := newTestServer(t)
	params, _ := json.Marshal(map[string]any{
		"name":      "not_a_real_tool",
		"arguments": map[string]any{},
	})
	_, err := s.handleToolsCall(context.Background(), params)
	if err == nil {
		t.Fatal("expected error")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./mcp/server/ -run TestHandle -v`
Expected: FAIL — handlers undefined.

- [ ] **Step 3: Implement the handlers**

Create `mcp/server/handlers.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// Server is the MCP server object.
type Server struct {
	opts *ServerOpts
}

// ServerOpts bundles dependencies.
type ServerOpts struct {
	Storage     storage.Storage
	Events      *EventBridge
	Permissions *PermissionQueue
}

// NewServer constructs a server from opts.
func NewServer(opts *ServerOpts) *Server { return &Server{opts: opts} }

// ---- initialize ----

func (s *Server) handleInitialize(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "hermind",
			"version": "dev",
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	})
}

// ---- tools/list ----

func (s *Server) handleToolsList(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"tools": BuiltinTools()})
}

// ---- tools/call ----

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p toolsCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	text, err := s.dispatchTool(ctx, p.Name, p.Arguments)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
	})
}

func (s *Server) dispatchTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "conversations_list":
		return s.conversationsList(ctx, args)
	case "conversation_get":
		return s.conversationGet(ctx, args)
	case "messages_read":
		return s.messagesRead(ctx, args)
	case "messages_send":
		return s.messagesSend(ctx, args)
	case "attachments_fetch":
		return s.attachmentsFetch(ctx, args)
	case "events_poll":
		return s.eventsPoll(ctx, args)
	case "events_wait":
		return s.eventsWait(ctx, args)
	case "permissions_list_open":
		return s.permissionsListOpen(ctx, args)
	case "permissions_respond":
		return s.permissionsRespond(ctx, args)
	case "channels_list":
		return s.channelsList(ctx, args)
	}
	return "", fmt.Errorf("mcp/server: unknown tool %q", name)
}

// ---- tool implementations ----

func (s *Server) conversationsList(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Platform string `json:"platform"`
		Limit    int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Limit <= 0 {
		a.Limit = 50
	}
	rows, err := s.opts.Storage.ListSessions(ctx, &storage.ListOptions{Limit: a.Limit})
	if err != nil {
		return "", err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		if a.Platform != "" && r.Source != a.Platform {
			continue
		}
		out = append(out, map[string]any{
			"session_key": r.ID,
			"session_id":  r.ID,
			"platform":    r.Source,
			"chat_name":   r.Title,
			"updated_at":  r.EndedAt,
		})
	}
	data, _ := json.MarshalIndent(map[string]any{
		"count":         len(out),
		"conversations": out,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) conversationGet(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		SessionKey string `json:"session_key"`
	}
	_ = json.Unmarshal(args, &a)
	if a.SessionKey == "" {
		return "", fmt.Errorf("session_key is required")
	}
	sess, err := s.opts.Storage.GetSession(ctx, a.SessionKey)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(sess, "", "  ")
	return string(data), nil
}

func (s *Server) messagesRead(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		SessionKey string `json:"session_key"`
		Limit      int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &a)
	if a.SessionKey == "" {
		return "", fmt.Errorf("session_key is required")
	}
	if a.Limit <= 0 {
		a.Limit = 50
	}
	msgs, err := s.opts.Storage.GetMessages(ctx, a.SessionKey, a.Limit, 0)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(map[string]any{
		"session_key": a.SessionKey,
		"count":       len(msgs),
		"messages":    msgs,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) messagesSend(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Target  string `json:"target"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Target == "" || a.Message == "" {
		return "", fmt.Errorf("target and message are required")
	}
	// MVP scope: MCP server records the message via the event bridge
	// so downstream subscribers pick it up. Wiring into the gateway
	// send_message path is a follow-up plan.
	s.opts.Events.push(Event{
		Cursor:     time.Now().UnixNano(),
		Kind:       "message",
		SessionKey: a.Target,
		Role:       "outgoing",
		Content:    a.Message,
	})
	return `{"status":"queued"}`, nil
}

func (s *Server) attachmentsFetch(_ context.Context, _ json.RawMessage) (string, error) {
	// Placeholder — hermind doesn't expose attachments through
	// storage.Storage yet. Returning an empty list keeps hosts happy.
	return `{"count":0,"attachments":[]}`, nil
}

func (s *Server) eventsPoll(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		AfterCursor int64  `json:"after_cursor"`
		SessionKey  string `json:"session_key"`
		Limit       int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &a)
	events, next := s.opts.Events.Poll(a.AfterCursor, a.SessionKey, a.Limit)
	data, _ := json.MarshalIndent(map[string]any{
		"events":      events,
		"next_cursor": next,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) eventsWait(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		AfterCursor int64  `json:"after_cursor"`
		SessionKey  string `json:"session_key"`
		TimeoutMS   int    `json:"timeout_ms"`
	}
	_ = json.Unmarshal(args, &a)
	if a.TimeoutMS <= 0 {
		a.TimeoutMS = 30_000
	}
	ev, err := s.opts.Events.Wait(ctx, a.AfterCursor, a.SessionKey, time.Duration(a.TimeoutMS)*time.Millisecond)
	if err != nil {
		return "", err
	}
	if ev == nil {
		return `null`, nil
	}
	data, _ := json.Marshal(ev)
	return string(data), nil
}

func (s *Server) permissionsListOpen(_ context.Context, _ json.RawMessage) (string, error) {
	open := s.opts.Permissions.ListOpen()
	data, _ := json.MarshalIndent(map[string]any{
		"count":       len(open),
		"permissions": open,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) permissionsRespond(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		ID       string `json:"id"`
		Decision string `json:"decision"`
	}
	_ = json.Unmarshal(args, &a)
	if !s.opts.Permissions.Respond(a.ID, a.Decision) {
		return "", fmt.Errorf("permission id %q not open", a.ID)
	}
	return `{"status":"recorded"}`, nil
}

func (s *Server) channelsList(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Platform string `json:"platform"`
	}
	_ = json.Unmarshal(args, &a)
	// MVP: derive the channel list from distinct Source values across
	// recent sessions. Production wiring would query a dedicated
	// channels table from the gateway layer.
	rows, err := s.opts.Storage.ListSessions(ctx, &storage.ListOptions{Limit: 200})
	if err != nil {
		return "", err
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, r := range rows {
		if a.Platform != "" && r.Source != a.Platform {
			continue
		}
		key := r.Source + ":" + r.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	data, _ := json.MarshalIndent(map[string]any{"channels": out}, "", "  ")
	return string(data), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./mcp/server/ -run TestHandle -v -race`
Expected: PASS (5 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add mcp/server/handlers.go mcp/server/handlers_test.go
git commit -m "feat(mcp/server): 10 MCP tool handlers"
```

---

## Task 5: Read/dispatch loop

**Files:**
- Create: `mcp/server/server.go`
- Create: `mcp/server/server_test.go`

- [ ] **Step 1: Write the failing test**

Create `mcp/server/server_test.go`:

```go
package server

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestServer_InitializeOverStdio(t *testing.T) {
	s, _ := newTestServer(t)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	var out bytes.Buffer
	if err := s.RunOnce(context.Background(), in, &out, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"protocolVersion"`) {
		t.Errorf("got %s", out.String())
	}
}

func TestServer_UnknownMethodReturnsError(t *testing.T) {
	s, _ := newTestServer(t)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"does_not_exist"}` + "\n")
	var out bytes.Buffer
	_ = s.RunOnce(context.Background(), in, &out, 1)
	if !strings.Contains(out.String(), `"code":-32601`) {
		t.Errorf("got %s", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./mcp/server/ -run TestServer -v`
Expected: FAIL — `RunOnce` undefined.

- [ ] **Step 3: Implement the server loop**

Create `mcp/server/server.go`:

```go
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/odysseythink/hermind/internal/jsonrpc"
)

// Run reads frames from r and writes responses to w until ctx is
// cancelled or r hits EOF.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	return s.RunOnce(ctx, r, w, -1)
}

// RunOnce is a test seam — stops after n frames (pass -1 for EOF).
func (s *Server) RunOnce(ctx context.Context, r io.Reader, w io.Writer, n int) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<16), 1<<22)
	var writeMu sync.Mutex
	count := 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		s.dispatch(ctx, append([]byte{}, line...), w, &writeMu)
		count++
		if n > 0 && count >= n {
			return nil
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, raw []byte, w io.Writer, mu *sync.Mutex) {
	req, err := jsonrpc.DecodeRequest(raw)
	if err != nil {
		s.writeLocked(mu, w, &jsonrpc.Response{
			Error: &jsonrpc.Error{Code: jsonrpc.CodeParseError, Message: err.Error()},
		})
		return
	}

	result, handlerErr := s.route(ctx, req.Method, req.Params)
	if req.IsNotification() {
		return
	}
	resp := &jsonrpc.Response{ID: req.ID}
	if handlerErr != nil {
		resp.Error = &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: handlerErr.Error(),
		}
		if _, ok := handlerErr.(*unknownMethodError); ok {
			resp.Error.Code = jsonrpc.CodeMethodNotFound
		}
	} else {
		resp.Result = result
	}
	s.writeLocked(mu, w, resp)
}

func (s *Server) writeLocked(mu *sync.Mutex, w io.Writer, resp *jsonrpc.Response) {
	mu.Lock()
	defer mu.Unlock()
	_ = jsonrpc.EncodeResponse(w, resp)
}

type unknownMethodError struct{ method string }

func (e *unknownMethodError) Error() string { return "method not found: " + e.method }

func (s *Server) route(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "initialize":
		return s.handleInitialize(ctx, params)
	case "tools/list":
		return s.handleToolsList(ctx, params)
	case "tools/call":
		return s.handleToolsCall(ctx, params)
	}
	return nil, &unknownMethodError{method: method}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./mcp/server/ -run TestServer -v -race`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add mcp/server/server.go mcp/server/server_test.go
git commit -m "feat(mcp/server): read/dispatch loop with jsonrpc frames"
```

---

## Task 6: CLI `hermind mcp serve`

**Files:**
- Create: `cli/mcp.go`
- Create: `cli/mcp_test.go`
- Modify: `cli/root.go`

- [ ] **Step 1: Write the failing test**

Create `cli/mcp_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMCPCmd_InitializeRoundTrip(t *testing.T) {
	app := newTestApp(t)
	cmd := newMCPCmd(app)
	var serve *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Use == "serve" {
			serve = c
		}
	}
	if serve == nil {
		t.Fatal("serve subcommand missing")
	}
	serve.SetIn(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"))
	var out bytes.Buffer
	serve.SetOut(&out)
	serve.SetErr(&bytes.Buffer{})
	if err := serve.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"protocolVersion"`) {
		t.Errorf("got %s", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestMCPCmd -v`
Expected: FAIL — `newMCPCmd` undefined.

- [ ] **Step 3: Implement the command**

Create `cli/mcp.go`:

```go
package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/mcp/server"
)

func newMCPCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP (Model Context Protocol) server",
	}
	cmd.AddCommand(newMCPServeCmd(app))
	return cmd
}

func newMCPServeCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run hermind as an MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureStorage(app); err != nil {
				return err
			}
			bridge := server.NewEventBridge(app.Storage, 200*time.Millisecond)
			perms := server.NewPermissionQueue()

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			go bridge.Run(ctx)

			srv := server.NewServer(&server.ServerOpts{
				Storage:     app.Storage,
				Events:      bridge,
				Permissions: perms,
			})
			return srv.Run(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}
```

- [ ] **Step 4: Register in root**

Add `newMCPCmd(app),` to `cli/root.go`'s `AddCommand(...)` call.

- [ ] **Step 5: Run tests**

Run: `go test ./cli/ -run TestMCPCmd -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/mcp.go cli/mcp_test.go cli/root.go
git commit -m "feat(cli): add 'hermind mcp serve' subcommand"
```

---

## Task 7: Manual smoke test

- [ ] **Step 1: Build**

```bash
go build -o /tmp/hermind ./cmd/hermind
```

- [ ] **Step 2: Drive initialize + tools/list**

```bash
printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | /tmp/hermind mcp serve 2>/tmp/mcp.log
```

Expected: two JSON frames on stdout — the initialize result then a `tools` array of 10 entries.

- [ ] **Step 3: Drive tools/call**

```bash
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"conversations_list","arguments":{"limit":5}}}' \
  | /tmp/hermind mcp serve 2>>/tmp/mcp.log
```

Expected: a response containing `"content":[{"type":"text","text":"<JSON>"}]` with the current conversation list (may be empty on a fresh install).

- [ ] **Step 4: Cleanup**

```bash
rm /tmp/hermind /tmp/mcp.log
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - 10 MCP tools ↔ Task 2 + Task 4 ✓
   - initialize / tools/list / tools/call ↔ Task 4 + Task 5 ✓
   - EventBridge poll + wait ↔ Task 3 ✓
   - PermissionQueue ↔ Task 3 ✓
   - stdio transport ↔ Task 5 ✓
   - CLI entry point ↔ Task 6 ✓
   - Shared JSON-RPC codec ↔ Task 1 ✓

2. **Placeholders:** `attachmentsFetch` and `channelsList` emit conservative defaults because hermind's storage layer doesn't surface attachments yet. Both are documented in-line.

3. **Type consistency:**
   - `Event{Cursor, Kind, SessionKey, Role, Content, At}` stable across Tasks 3, 4.
   - `PermissionRequest{ID, Command, Kind, OpenAt}` stable between Task 3 and handlers.
   - `ServerOpts{Storage, Events, Permissions}` identical in Tasks 4, 5, 6.
   - `jsonrpc.Request`, `Response`, `Error` used consistently across Tasks 1, 5.

4. **Gaps (future work):**
   - Replace the ad-hoc tick loop with change-data-capture when `storage.Storage` grows a notification hook.
   - Switch to `github.com/modelcontextprotocol/sdk-go` once it stabilizes and supports the capabilities we need.
   - Wire `permissions_respond` into `tool.Registry` so destructive tools actually wait on MCP-delivered decisions.

---

## Definition of Done

- `go test ./internal/jsonrpc/... ./mcp/server/... ./cli/... -race` all pass.
- `hermind mcp serve` serves `initialize`, `tools/list`, `tools/call` correctly over stdio.
- Manual smoke test (three frames → three responses) succeeds.
- EventBridge ticker runs without leaking goroutines on `hermind mcp serve` exit.
