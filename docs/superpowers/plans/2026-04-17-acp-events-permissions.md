# ACP Events, Permissions & Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Depends on:** Plan D (ACP stdio MVP) — the `acp/stdio` package, `Handlers`, `SessionManager`, and `Server` types must already exist.

**Goal:** Bring the Go ACP stdio server to feature parity with the Python `acp_adapter` so Zed/Cursor/VS Code see streaming agent output, tool-call progress, permission prompts before destructive tool execution, and a correctly populated `.well-known/agent.json` registry. Also add the remaining session verbs (`fork_session`, `list_sessions`, `set_session_model`, `set_session_mode`).

**Architecture:** A new `EventBus` carries server-initiated notifications out of the prompt handler while it runs; a `Notifier` wrapper marshals `session/update` frames onto stdout through the existing `writeMu` lock so concurrent handlers never interleave JSON lines. A `PermissionBroker` bridges hermind's `tool.ApprovalFunc` (called from `tool.Registry`) to the ACP `session/request_permission` round-trip, with a 60-second timeout → auto-deny per the Python reference behavior. The registry JSON is generated at `hermind acp registry` subcommand time and also served by a new `-P` flag that prints it to stdout (used by IDE installers).

**Tech Stack:** Go 1.21+, existing `acp/stdio`, `tool`, `provider`, `message`, `storage`, `cobra` packages. `github.com/google/uuid` for tool-call IDs.

---

## File Structure

- Create: `acp/stdio/events.go` — `EventBus`, `Notifier`, typed update payloads
- Create: `acp/stdio/events_test.go`
- Create: `acp/stdio/permissions.go` — `PermissionBroker` with request/response correlation
- Create: `acp/stdio/permissions_test.go`
- Create: `acp/stdio/registry.go` — `BuildRegistry` producing `.well-known/agent.json` JSON bytes
- Create: `acp/stdio/registry_test.go`
- Modify: `acp/stdio/handlers.go` — extend `Handlers` with `Notifier` and `PermissionBroker`; stream deltas during `prompt`; add `fork_session`, `list_sessions`, `set_session_model`, `set_session_mode`
- Modify: `acp/stdio/handlers_test.go`
- Modify: `acp/stdio/server.go` — thread the notifier through dispatch so the server writes both responses and notifications through the same lock; route the new session verbs
- Modify: `cli/acp.go` — wire EventBus + PermissionBroker into `Handlers`; add `hermind acp registry` subcommand

---

## Task 1: EventBus + Notifier

**Files:**
- Create: `acp/stdio/events.go`
- Create: `acp/stdio/events_test.go`

- [ ] **Step 1: Write the failing test**

Create `acp/stdio/events_test.go`:

```go
package stdio

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNotifier_AgentMessageChunk(t *testing.T) {
	var out bytes.Buffer
	n := NewNotifier(&out, nil)
	n.AgentMessageChunk("s1", "hel")
	n.AgentMessageChunk("s1", "lo")
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 frames, got %d: %q", len(lines), out.String())
	}
	for _, l := range lines {
		if !strings.Contains(l, `"method":"session/update"`) {
			t.Errorf("missing method: %s", l)
		}
		if !strings.Contains(l, `"sessionUpdate":"agent_message_chunk"`) {
			t.Errorf("missing update kind: %s", l)
		}
	}
}

func TestNotifier_ToolCallStartAndUpdate(t *testing.T) {
	var out bytes.Buffer
	n := NewNotifier(&out, nil)
	id, _ := n.ToolCallStart("s1", "read_file", "execute", `{"path":"/tmp/x"}`)
	if id == "" {
		t.Fatal("empty tool call id")
	}
	n.ToolCallUpdate("s1", id, "completed", "contents here")

	frames := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	var start map[string]any
	_ = json.Unmarshal([]byte(frames[0]), &start)
	params := start["params"].(map[string]any)
	update := params["update"].(map[string]any)
	if update["sessionUpdate"] != "tool_call_start" {
		t.Errorf("first frame = %v", update["sessionUpdate"])
	}
	if update["toolCallId"] != id {
		t.Errorf("id mismatch: %v vs %v", update["toolCallId"], id)
	}
}

func TestNotifier_SerializedWrites(t *testing.T) {
	var out bytes.Buffer
	n := NewNotifier(&out, nil)
	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func() {
			n.AgentMessageChunk("s", "x")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
	// Every line must parse — if writes interleaved, some would be garbled.
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var tmp map[string]any
		if err := json.Unmarshal([]byte(l), &tmp); err != nil {
			t.Errorf("corrupt frame: %q", l)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run TestNotifier -v`
Expected: FAIL — `NewNotifier` undefined.

- [ ] **Step 3: Implement EventBus + Notifier**

Create `acp/stdio/events.go`:

```go
package stdio

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"
)

// Notifier emits session/update JSON-RPC notification frames. It is
// safe for concurrent use — writes are serialized on an internal
// mutex so frames never interleave.
type Notifier struct {
	mu  *sync.Mutex
	out io.Writer
}

// NewNotifier wraps w. If sharedMu is non-nil the notifier uses it
// instead of allocating a fresh lock — this is how the Server shares
// its stdout lock with the notifier so responses and notifications
// never interleave.
func NewNotifier(w io.Writer, sharedMu *sync.Mutex) *Notifier {
	if sharedMu == nil {
		sharedMu = &sync.Mutex{}
	}
	return &Notifier{mu: sharedMu, out: w}
}

// AgentMessageChunk emits a streaming text chunk from the assistant.
func (n *Notifier) AgentMessageChunk(sessionID, text string) {
	n.send(sessionID, map[string]any{
		"sessionUpdate":     "agent_message_chunk",
		"agentMessageChunk": map[string]string{"text": text},
	})
}

// AgentThoughtChunk emits a "thinking" text chunk (distinct from
// visible message content).
func (n *Notifier) AgentThoughtChunk(sessionID, text string) {
	n.send(sessionID, map[string]any{
		"sessionUpdate":     "agent_thought_chunk",
		"agentThoughtChunk": map[string]string{"text": text},
	})
}

// ToolCallStart emits a tool_call_start update and returns the
// generated tool call ID. Callers pass it into ToolCallUpdate when
// the tool completes.
func (n *Notifier) ToolCallStart(sessionID, toolName, kind string, rawInput string) (string, error) {
	id, err := makeToolCallID()
	if err != nil {
		return "", err
	}
	n.send(sessionID, map[string]any{
		"sessionUpdate": "tool_call_start",
		"toolCallId":    id,
		"kind":          kind,
		"title":         toolName,
		"rawInput":      rawInput,
	})
	return id, nil
}

// ToolCallUpdate emits a tool_call_update when a tool finishes (or
// fails). status is usually "completed" or "failed".
func (n *Notifier) ToolCallUpdate(sessionID, toolCallID, status, rawOutput string) {
	n.send(sessionID, map[string]any{
		"sessionUpdate": "tool_call_update",
		"toolCallId":    toolCallID,
		"status":        status,
		"rawOutput":     rawOutput,
	})
}

// AvailableCommands emits the list of slash commands the agent exposes
// (e.g. /reset, /compact). Sent once per session after new/load/resume.
func (n *Notifier) AvailableCommands(sessionID string, commands []string) {
	cmdObjs := make([]map[string]string, 0, len(commands))
	for _, c := range commands {
		cmdObjs = append(cmdObjs, map[string]string{"name": c})
	}
	n.send(sessionID, map[string]any{
		"sessionUpdate":     "available_commands_update",
		"availableCommands": cmdObjs,
	})
}

func (n *Notifier) send(sessionID string, update map[string]any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	_ = EncodeNotification(n.out, "session/update", map[string]any{
		"sessionId": sessionID,
		"update":    update,
	})
}

// makeToolCallID returns "tc-" + 12 hex chars, matching the Python
// helper's shape so existing test expectations line up.
func makeToolCallID() (string, error) {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "tc-" + hex.EncodeToString(buf[:]), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run TestNotifier -v -race`
Expected: PASS (3 sub-tests, no races).

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/events.go acp/stdio/events_test.go
git commit -m "feat(acp/stdio): Notifier for session/update streaming"
```

---

## Task 2: PermissionBroker

**Files:**
- Create: `acp/stdio/permissions.go`
- Create: `acp/stdio/permissions_test.go`

- [ ] **Step 1: Write the failing test**

Create `acp/stdio/permissions_test.go`:

```go
package stdio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type fakeRPC struct {
	outbox chan []byte
	inbox  chan []byte
}

func (f *fakeRPC) Send(data []byte) { f.outbox <- data }

func TestPermissionBroker_AllowOnce(t *testing.T) {
	outbox := make(chan []byte, 4)
	broker := NewPermissionBroker(func(data []byte) { outbox <- data }, 1*time.Second)

	// Start a request in the background.
	outcomeCh := make(chan PermissionOutcome, 1)
	go func() {
		oc, _ := broker.Request(context.Background(), "s1", "run rm -rf /tmp/x", "execute")
		outcomeCh <- oc
	}()

	// Read the outgoing JSON-RPC frame.
	frame := <-outbox
	var req struct {
		ID     json.Number     `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	_ = json.Unmarshal(frame, &req)
	if req.Method != "session/request_permission" {
		t.Errorf("method = %q", req.Method)
	}

	// Feed back a response.
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]any{
			"outcome": map[string]any{"optionId": "allow_once"},
		},
	}
	raw, _ := json.Marshal(resp)
	broker.HandleResponse(raw)

	select {
	case oc := <-outcomeCh:
		if oc != PermissionAllowOnce {
			t.Errorf("outcome = %v", oc)
		}
	case <-time.After(time.Second):
		t.Fatal("Request did not return")
	}
}

func TestPermissionBroker_TimeoutDenies(t *testing.T) {
	outbox := make(chan []byte, 4)
	broker := NewPermissionBroker(func(data []byte) { outbox <- data }, 50*time.Millisecond)

	oc, err := broker.Request(context.Background(), "s1", "ls", "execute")
	if err != nil && !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout, got %v", err)
	}
	if oc != PermissionDeny {
		t.Errorf("outcome = %v", oc)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run TestPermissionBroker -v`
Expected: FAIL — `PermissionBroker` undefined.

- [ ] **Step 3: Implement the broker**

Create `acp/stdio/permissions.go`:

```go
package stdio

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PermissionOutcome mirrors the ACP response.outcome.optionId values.
type PermissionOutcome int

const (
	PermissionDeny PermissionOutcome = iota
	PermissionAllowOnce
	PermissionAllowAlways
)

// String returns the camel-case form used in ACP messages.
func (p PermissionOutcome) String() string {
	switch p {
	case PermissionAllowOnce:
		return "allow_once"
	case PermissionAllowAlways:
		return "allow_always"
	default:
		return "deny"
	}
}

// PermissionBroker issues session/request_permission calls and waits
// for matching responses. It is the bridge between hermind's
// tool.ApprovalFunc (synchronous) and the ACP client (async).
type PermissionBroker struct {
	send    func([]byte)
	timeout time.Duration

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan PermissionOutcome
}

// NewPermissionBroker constructs a broker. send is the function the
// server uses to write outbound JSON-RPC frames (typically closed
// over the Notifier's stdout + mutex).
func NewPermissionBroker(send func([]byte), timeout time.Duration) *PermissionBroker {
	return &PermissionBroker{
		send:    send,
		timeout: timeout,
		pending: map[int64]chan PermissionOutcome{},
	}
}

// Request sends a permission request and blocks until the client
// responds or the timeout fires. Timeout maps to PermissionDeny so
// the agent stays safe-by-default when the editor is slow/absent.
func (b *PermissionBroker) Request(ctx context.Context, sessionID, description, kind string) (PermissionOutcome, error) {
	id := atomic.AddInt64(&b.nextID, 1)
	ch := make(chan PermissionOutcome, 1)

	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()

	frame, err := json.Marshal(struct {
		Version string      `json:"jsonrpc"`
		ID      int64       `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
	}{
		Version: "2.0",
		ID:      id,
		Method:  "session/request_permission",
		Params: map[string]any{
			"sessionId": sessionID,
			"toolCall": map[string]any{
				"toolCallId": "perm-check",
				"title":      description,
				"kind":       kind,
			},
			"options": []map[string]string{
				{"optionId": "allow_once", "kind": "allow_once", "name": "Allow once"},
				{"optionId": "allow_always", "kind": "allow_always", "name": "Allow always"},
				{"optionId": "deny", "kind": "reject_once", "name": "Deny"},
			},
		},
	})
	if err != nil {
		return PermissionDeny, err
	}
	// Append the newline just like EncodeResponse.
	frame = append(frame, '\n')
	b.send(frame)

	select {
	case outcome := <-ch:
		return outcome, nil
	case <-time.After(b.timeout):
		b.clear(id)
		return PermissionDeny, fmt.Errorf("acp/stdio: permission request timeout after %s", b.timeout)
	case <-ctx.Done():
		b.clear(id)
		return PermissionDeny, ctx.Err()
	}
}

// HandleResponse feeds an incoming JSON-RPC response back into the
// waiting Request. It is a no-op if the ID is not pending.
func (b *PermissionBroker) HandleResponse(raw []byte) {
	var resp struct {
		ID     int64 `json:"id"`
		Result struct {
			Outcome struct {
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}
	outcome := PermissionDeny
	switch resp.Result.Outcome.OptionID {
	case "allow_once":
		outcome = PermissionAllowOnce
	case "allow_always":
		outcome = PermissionAllowAlways
	}

	b.mu.Lock()
	ch, ok := b.pending[resp.ID]
	delete(b.pending, resp.ID)
	b.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- outcome:
	default:
		// Already signaled (timeout path); drop.
	}
}

func (b *PermissionBroker) clear(id int64) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run TestPermissionBroker -v -race`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/permissions.go acp/stdio/permissions_test.go
git commit -m "feat(acp/stdio): PermissionBroker with 60s timeout deny-by-default"
```

---

## Task 3: Registry (`.well-known/agent.json`)

**Files:**
- Create: `acp/stdio/registry.go`
- Create: `acp/stdio/registry_test.go`

- [ ] **Step 1: Write the failing test**

Create `acp/stdio/registry_test.go`:

```go
package stdio

import (
	"encoding/json"
	"testing"
)

func TestBuildRegistry_DefaultShape(t *testing.T) {
	data := BuildRegistry(RegistryOpts{
		Version: "1.2.3",
	})
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "hermind" {
		t.Errorf("name = %v", got["name"])
	}
	if got["display_name"] != "Hermind" {
		t.Errorf("display_name = %v", got["display_name"])
	}
	dist, _ := got["distribution"].(map[string]any)
	if dist["type"] != "command" {
		t.Errorf("distribution.type = %v", dist["type"])
	}
	if dist["command"] != "hermind" {
		t.Errorf("distribution.command = %v", dist["command"])
	}
	args, _ := dist["args"].([]any)
	if len(args) != 1 || args[0] != "acp" {
		t.Errorf("distribution.args = %v", args)
	}
}

func TestBuildRegistry_CustomBinary(t *testing.T) {
	data := BuildRegistry(RegistryOpts{
		Version:    "1.0.0",
		BinaryPath: "/usr/local/bin/hermind-dev",
	})
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	dist := got["distribution"].(map[string]any)
	if dist["command"] != "/usr/local/bin/hermind-dev" {
		t.Errorf("command = %v", dist["command"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run TestBuildRegistry -v`
Expected: FAIL — `BuildRegistry` undefined.

- [ ] **Step 3: Implement the registry builder**

Create `acp/stdio/registry.go`:

```go
package stdio

import "encoding/json"

// RegistryOpts controls the generated agent.json content.
type RegistryOpts struct {
	Version    string // hermind version string
	BinaryPath string // path ACP clients should launch; "" → "hermind"
	IconURL    string // optional URL to agent icon
}

// BuildRegistry returns the JSON body of the agent.json registry
// entry. Writing it to disk is the caller's responsibility.
func BuildRegistry(opts RegistryOpts) []byte {
	cmd := opts.BinaryPath
	if cmd == "" {
		cmd = "hermind"
	}
	entry := map[string]any{
		"schema_version": 1,
		"name":           "hermind",
		"display_name":   "Hermind",
		"version":        opts.Version,
		"description":    "Go port of the hermes AI agent framework.",
		"distribution": map[string]any{
			"type":    "command",
			"command": cmd,
			"args":    []string{"acp"},
		},
	}
	if opts.IconURL != "" {
		entry["icon"] = opts.IconURL
	}
	out, _ := json.MarshalIndent(entry, "", "  ")
	return append(out, '\n')
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run TestBuildRegistry -v`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/registry.go acp/stdio/registry_test.go
git commit -m "feat(acp/stdio): BuildRegistry for .well-known/agent.json"
```

---

## Task 4: Wire Notifier into Handlers (streaming prompt)

**Files:**
- Modify: `acp/stdio/handlers.go`
- Modify: `acp/stdio/handlers_test.go`

- [ ] **Step 1: Write the failing test**

Append to `acp/stdio/handlers_test.go`:

```go
func TestHandlePrompt_StreamsChunks(t *testing.T) {
	h := newTestHandlers(t)

	var out bytes.Buffer
	h.Notifier = NewNotifier(&out, nil)

	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")
	params, _ := json.Marshal(map[string]any{
		"sessionId": s.ID,
		"prompt":    []any{map[string]any{"type": "text", "text": "hi"}},
	})
	_, err := h.handlePrompt(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	// The stub provider returns a single "ok" message, so at least
	// one agent_message_chunk frame must be visible.
	if !bytes.Contains(out.Bytes(), []byte(`"sessionUpdate":"agent_message_chunk"`)) {
		t.Errorf("missing agent_message_chunk: %s", out.String())
	}
}
```

Make sure `"bytes"` and `"encoding/json"` are imported at the top of the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run TestHandlePrompt_StreamsChunks -v`
Expected: FAIL — `h.Notifier` undefined.

- [ ] **Step 3: Extend Handlers**

In `acp/stdio/handlers.go`, update the `Handlers` struct:

```go
// Handlers bundles the collaborators each ACP method needs.
type Handlers struct {
	Sessions *SessionManager
	Factory  func(model string) (provider.Provider, error)
	AgentCfg config.AgentConfig

	// Notifier streams session/update notifications during prompt
	// execution. May be nil in tests that do not care about streaming.
	Notifier *Notifier

	// Perms issues session/request_permission calls before destructive
	// tool execution. May be nil to allow all.
	Perms *PermissionBroker
}
```

Then update `handlePrompt` to emit a chunk after the provider returns (MVP streaming — streaming deltas from `Stream()` are a later upgrade):

```go
func (h *Handlers) handlePrompt(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p promptParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, errors.New("acp/stdio: prompt: sessionId is required")
	}
	s, err := h.Sessions.Get(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	text := extractText(p.Prompt)
	if text == "" {
		return json.Marshal(promptResult{StopReason: "refusal"})
	}
	if err := h.Sessions.AppendUserText(ctx, s.ID, text); err != nil {
		return nil, err
	}

	prov, err := h.Factory(s.Model)
	if err != nil {
		return nil, fmt.Errorf("acp/stdio: provider factory: %w", err)
	}
	history, err := h.Sessions.History(ctx, s.ID)
	if err != nil {
		return nil, err
	}

	cctx, cancel := context.WithCancel(ctx)
	h.Sessions.SetCancel(s.ID, cancel)
	defer cancel()

	resp, err := prov.Complete(cctx, &provider.Request{
		Model:     s.Model,
		Messages:  history,
		MaxTokens: 4096,
	})
	if err != nil {
		if errors.Is(cctx.Err(), context.Canceled) {
			return json.Marshal(promptResult{StopReason: "cancelled"})
		}
		return nil, err
	}

	reply := resp.Message.Content.Text()
	if h.Notifier != nil && reply != "" {
		h.Notifier.AgentMessageChunk(s.ID, reply)
	}
	if err := h.Sessions.AppendAssistantText(ctx, s.ID, reply); err != nil {
		return nil, err
	}

	stop := resp.FinishReason
	if stop == "" {
		stop = "end_turn"
	}
	return json.Marshal(promptResult{StopReason: stop})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run TestHandlePrompt_StreamsChunks -v`
Expected: PASS.

Run all stdio tests to confirm no regressions:

Run: `go test ./acp/stdio/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/handlers.go acp/stdio/handlers_test.go
git commit -m "feat(acp/stdio): emit agent_message_chunk during prompt"
```

---

## Task 5: Additional session verbs

**Files:**
- Modify: `acp/stdio/handlers.go`
- Modify: `acp/stdio/handlers_test.go`
- Modify: `acp/stdio/server.go` — route the new methods
- Modify: `acp/stdio/session.go` — add Fork/List/SetModel helpers

- [ ] **Step 1: Write the failing test**

Append to `acp/stdio/handlers_test.go`:

```go
func TestHandleForkSession_CopiesHistory(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")
	_ = h.Sessions.AppendUserText(context.Background(), s.ID, "hello")

	params, _ := json.Marshal(map[string]any{
		"cwd":       "/tmp",
		"sessionId": s.ID,
	})
	raw, err := h.handleForkSession(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	_ = json.Unmarshal(raw, &resp)
	if resp["sessionId"] == "" || resp["sessionId"] == s.ID {
		t.Fatalf("fork returned bad id: %v", resp)
	}
	// History must be copied.
	msgs, _ := h.Sessions.History(context.Background(), resp["sessionId"])
	if len(msgs) != 1 || msgs[0].Content.Text() != "hello" {
		t.Errorf("forked history = %+v", msgs)
	}
}

func TestHandleListSessions(t *testing.T) {
	h := newTestHandlers(t)
	_, _ = h.Sessions.Create(context.Background(), "/tmp", "m1")
	_, _ = h.Sessions.Create(context.Background(), "/tmp", "m2")

	raw, err := h.handleListSessions(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	_ = json.Unmarshal(raw, &resp)
	sessions, _ := resp["sessions"].([]any)
	if len(sessions) < 2 {
		t.Errorf("sessions = %v", sessions)
	}
}

func TestHandleSetSessionModel(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "m1")

	params, _ := json.Marshal(map[string]any{
		"sessionId": s.ID,
		"modelId":   "m2",
	})
	if _, err := h.handleSetSessionModel(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	after, _ := h.Sessions.Get(context.Background(), s.ID)
	if after.Model != "m2" {
		t.Errorf("model = %q", after.Model)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run "TestHandleFork|TestHandleList|TestHandleSetSessionModel" -v`
Expected: FAIL — handlers undefined.

- [ ] **Step 3: Implement the handlers**

Append to `acp/stdio/handlers.go`:

```go
type forkSessionParams struct {
	Cwd       string `json:"cwd"`
	SessionID string `json:"sessionId"`
}

type sessionInfo struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd,omitempty"`
}

type setSessionModelParams struct {
	SessionID string `json:"sessionId"`
	ModelID   string `json:"modelId"`
}

func (h *Handlers) handleForkSession(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p forkSessionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	fresh, err := h.Sessions.Fork(ctx, p.SessionID, p.Cwd)
	if err != nil {
		return nil, err
	}
	return json.Marshal(newSessionResult{SessionID: fresh.ID})
}

func (h *Handlers) handleListSessions(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
	sess, err := h.Sessions.List(ctx)
	if err != nil {
		return nil, err
	}
	infos := make([]sessionInfo, 0, len(sess))
	for _, s := range sess {
		infos = append(infos, sessionInfo{SessionID: s.ID, Cwd: s.Cwd})
	}
	return json.Marshal(map[string]any{"sessions": infos})
}

func (h *Handlers) handleSetSessionModel(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p setSessionModelParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Sessions.SetModel(ctx, p.SessionID, p.ModelID); err != nil {
		return nil, err
	}
	return json.RawMessage(`{}`), nil
}
```

Append helpers to `acp/stdio/session.go`:

```go
// Fork copies a session's history into a fresh session.
func (m *SessionManager) Fork(ctx context.Context, srcID, cwd string) (*Session, error) {
	src, err := m.Get(ctx, srcID)
	if err != nil {
		return nil, err
	}
	dst, err := m.Create(ctx, cwd, src.Model)
	if err != nil {
		return nil, err
	}
	msgs, err := m.History(ctx, srcID)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgs {
		var role string
		if msg.Role == message.RoleUser {
			role = string(message.RoleUser)
		} else {
			role = string(message.RoleAssistant)
		}
		if err := m.store.AddMessage(ctx, dst.ID, &storage.StoredMessage{
			Role:    role,
			Content: msg.Content.Text(),
		}); err != nil {
			return nil, err
		}
	}
	return dst, nil
}

// List returns every in-memory session.
func (m *SessionManager) List(_ context.Context) ([]*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out, nil
}

// SetModel changes a session's active model, persisting the update.
func (m *SessionManager) SetModel(ctx context.Context, id, model string) error {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return fmt.Errorf("acp/stdio: unknown session %s", id)
	}
	s.Model = model
	// Storage.UpdateSession is interface-defined; pass a minimal update.
	return m.store.UpdateSession(ctx, id, &storage.SessionUpdate{Model: &model})
}
```

If `storage.SessionUpdate.Model` is not a `*string`, adjust the assignment to match the real struct (e.g. `&storage.SessionUpdate{Model: model}` with a plain string field). Run `grep -n "type SessionUpdate" storage/types.go` to confirm.

Add routing for the new methods in `acp/stdio/server.go` inside `route()`:

```go
	case "session/fork", "forkSession", "fork_session":
		return s.handlers.handleForkSession(ctx, params)
	case "session/list", "listSessions", "list_sessions":
		return s.handlers.handleListSessions(ctx, params)
	case "session/set_model", "setSessionModel", "set_session_model":
		return s.handlers.handleSetSessionModel(ctx, params)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run "TestHandleFork|TestHandleList|TestHandleSetSessionModel" -v`
Expected: PASS.

Run full suite: `go test ./acp/stdio/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/handlers.go acp/stdio/session.go acp/stdio/server.go acp/stdio/handlers_test.go
git commit -m "feat(acp/stdio): fork_session, list_sessions, set_session_model"
```

---

## Task 6: CLI wiring + `hermind acp registry`

**Files:**
- Modify: `cli/acp.go`
- Modify: `cli/acp_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cli/acp_test.go`:

```go
func TestACPRegistryCmd_PrintsAgentJSON(t *testing.T) {
	app := newTestApp(t)

	cmd := newACPCmd(app)
	// Locate the "registry" subcommand.
	var reg *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Use == "registry" {
			reg = c
		}
	}
	if reg == nil {
		t.Fatal("registry subcommand not found")
	}
	reg.SetArgs(nil)
	var out bytes.Buffer
	reg.SetOut(&out)
	reg.SetErr(&out)
	if err := reg.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"name": "hermind"`)) {
		t.Errorf("missing name field: %s", out.String())
	}
}
```

Add imports `"bytes"`, `"github.com/spf13/cobra"` to the test file if not already there.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestACPRegistryCmd -v`
Expected: FAIL — no registry subcommand.

- [ ] **Step 3: Extend the acp cobra tree**

Replace `newACPCmd` in `cli/acp.go` so it becomes a parent that also hosts `registry`:

```go
func newACPCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acp",
		Short: "Run hermind as an ACP stdio server (editor integration)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runACPStdio(cmd, app)
		},
	}
	cmd.AddCommand(newACPRegistryCmd())
	return cmd
}

func newACPRegistryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "registry",
		Short: "Print the agent.json registry entry for this binary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data := stdio.BuildRegistry(stdio.RegistryOpts{
				Version: Version,
			})
			_, err := cmd.OutOrStdout().Write(data)
			return err
		},
	}
}

func runACPStdio(cmd *cobra.Command, app *App) error {
	if err := ensureStorage(app); err != nil {
		return err
	}
	var writeMu sync.Mutex
	notifier := stdio.NewNotifier(cmd.OutOrStdout(), &writeMu)
	broker := stdio.NewPermissionBroker(func(frame []byte) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, _ = cmd.OutOrStdout().Write(frame)
	}, 60*time.Second)

	handlers := &stdio.Handlers{
		Sessions: stdio.NewSessionManager(app.Storage),
		Factory: func(model string) (provider.Provider, error) {
			cfg := resolveProviderConfig(app.Config, model)
			return factory.New(cfg)
		},
		AgentCfg: app.Config.Agent,
		Notifier: notifier,
		Perms:    broker,
	}
	srv := stdio.NewServer(handlers)
	return srv.Run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
}
```

Add imports `"sync"`, `"time"` to the file.

- [ ] **Step 4: Run tests**

Run: `go test ./cli/ -run TestACP -v`
Expected: PASS (both existing + new test).

- [ ] **Step 5: Commit**

```bash
git add cli/acp.go cli/acp_test.go
git commit -m "feat(cli): wire ACP Notifier + PermissionBroker; add 'hermind acp registry'"
```

---

## Task 7: Manual smoke test

- [ ] **Step 1: Build**

```bash
go build -o /tmp/hermind ./cmd/hermind
```

- [ ] **Step 2: Check the registry**

```bash
/tmp/hermind acp registry
```

Expected output: JSON object with `"name": "hermind"`, `"distribution.type": "command"`, `"distribution.command": "hermind"`, `"distribution.args": ["acp"]`.

- [ ] **Step 3: Drive a streaming prompt**

```bash
printf '%s\n%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/tmp"}}' \
  '{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"sessionId":"__fill__","prompt":[{"type":"text","text":"hi"}]}}' \
  | /tmp/hermind acp 2>/tmp/hermind-acp.log
```

(Mid-script `__fill__` has to be replaced with the session ID from frame #2; fine for manual run with a scratch script or two-step paste.)

Expected: stdout contains `session/update` notification frames with `"agent_message_chunk"` before the final `stopReason` response.

- [ ] **Step 4: Cleanup**

```bash
rm /tmp/hermind /tmp/hermind-acp.log
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - `session/update` notifications (agent_message_chunk, tool_call_*) ↔ Task 1 ✓
   - `session/request_permission` round-trip w/ timeout ↔ Task 2 ✓
   - `.well-known/agent.json` ↔ Task 3 ✓
   - `session/fork`, `session/list`, `session/set_model` ↔ Task 5 ✓
   - Streaming prompt emits chunks ↔ Task 4 ✓
   - Shared stdout lock prevents interleaving ↔ Task 1 (`Notifier.mu`) + Task 6 (shared `writeMu`) ✓

2. **Placeholders:** Task 5 Step 3 calls out a possible adjustment to `storage.SessionUpdate.Model` field shape (pointer vs value) with a concrete grep command — not a TBD, but a one-liner adjustment the engineer must confirm locally.

3. **Type consistency:**
   - `Notifier{mu, out}` with `NewNotifier(w, sharedMu)` signature used in Task 1 tests + Task 6 wiring.
   - `PermissionBroker.Request(ctx, sessionID, description, kind)` stable between Task 2 and future callers.
   - `Handlers{Sessions, Factory, AgentCfg, Notifier, Perms}` final shape stable across Tasks 4, 5, 6.
   - `Fork(ctx, srcID, cwd)` / `SetModel(ctx, id, model)` / `List(ctx)` method set stable.

4. **Gaps (future work):**
   - True provider `Stream()` integration so chunks arrive as the model emits them (MVP emits one final chunk after `Complete`).
   - `authMethods` populated from detected providers in `initialize`.
   - Tool execution actually calling `PermissionBroker.Request` — the wiring belongs in `tool.Registry` and is out of scope here.

---

## Definition of Done

- `go test ./acp/stdio/... ./cli/...` all pass.
- `hermind acp registry` prints a valid agent.json.
- Manual smoke test shows at least one `session/update` notification between session/new and the prompt's response.
- `PermissionBroker.Request` times out to `PermissionDeny` when the client is silent.
