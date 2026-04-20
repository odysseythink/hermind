# ACP stdio MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose hermind as a **stdio JSON-RPC 2.0 ACP server** so editors (Zed, Cursor, VS Code plugins) can drive the agent over stdin/stdout. MVP covers: `initialize`, `authenticate`, `new_session`, `load_session`, `prompt` (text only), `cancel`. Tool execution, permissions prompts, and session update streaming are scoped to Plan F.

**Architecture:** A new `acp/stdio/` package owns the protocol. It is distinct from the pre-existing HTTP-based `gateway/acp/` which stays untouched (different transport, different use case). The server reads newline-delimited JSON-RPC frames from stdin, dispatches each to a typed handler, and writes the result (or session_update notifications) back to stdout. A `SessionManager` persists sessions in the existing `storage.Storage` with `Source="acp"`. A `hermind acp` CLI subcommand wires stdin/stdout to the server and sends all logs to stderr so they never corrupt the protocol.

**Tech Stack:** Go 1.21+, `github.com/google/uuid`, `encoding/json`, existing `agent`, `provider/factory`, `storage`, `message`, `cobra` packages, no external JSON-RPC library (the protocol is tiny).

---

## File Structure

- Create: `acp/stdio/protocol.go` — JSON-RPC frame types and ACP request/response payloads
- Create: `acp/stdio/protocol_test.go`
- Create: `acp/stdio/session.go` — `Session` + `SessionManager` (in-memory cache + storage roundtrip)
- Create: `acp/stdio/session_test.go`
- Create: `acp/stdio/server.go` — read loop, dispatch, handler wiring
- Create: `acp/stdio/server_test.go`
- Create: `acp/stdio/handlers.go` — `initialize`, `authenticate`, `newSession`, `loadSession`, `prompt`, `cancel`
- Create: `acp/stdio/handlers_test.go`
- Create: `cli/acp.go` — `hermind acp` subcommand
- Create: `cli/acp_test.go`
- Modify: `cli/root.go` — register `newACPCmd(app)`

---

## Task 1: Protocol frame types

**Files:**
- Create: `acp/stdio/protocol.go`
- Create: `acp/stdio/protocol_test.go`

- [ ] **Step 1: Write the failing test**

Create `acp/stdio/protocol_test.go`:

```go
package stdio

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDecodeRequest_String(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":7,"method":"prompt","params":{"sessionId":"s1","prompt":[{"type":"text","text":"hi"}]}}`)
	req, err := DecodeRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "prompt" {
		t.Errorf("method = %q", req.Method)
	}
	var id json.Number
	_ = json.Unmarshal(req.ID, &id)
	if id != "7" {
		t.Errorf("id = %q", id)
	}
}

func TestEncodeResponse_ResultOnly(t *testing.T) {
	var buf bytes.Buffer
	resp := &Response{
		ID:     json.RawMessage(`7`),
		Result: json.RawMessage(`{"stopReason":"end_turn"}`),
	}
	if err := EncodeResponse(&buf, resp); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// Must be single-line terminated with \n.
	if got[len(got)-1] != '\n' {
		t.Error("missing trailing newline")
	}
	if !bytes.Contains([]byte(got), []byte(`"jsonrpc":"2.0"`)) {
		t.Errorf("missing jsonrpc marker: %s", got)
	}
	if bytes.Contains([]byte(got), []byte(`"error"`)) {
		t.Errorf("error present when Result set: %s", got)
	}
}

func TestEncodeResponse_ErrorOnly(t *testing.T) {
	var buf bytes.Buffer
	resp := &Response{
		ID: json.RawMessage(`7`),
		Error: &Error{
			Code:    -32601,
			Message: "method not found",
		},
	}
	_ = EncodeResponse(&buf, resp)
	if !bytes.Contains(buf.Bytes(), []byte(`"code":-32601`)) {
		t.Errorf("got %s", buf.String())
	}
}

func TestEncodeNotification(t *testing.T) {
	var buf bytes.Buffer
	_ = EncodeNotification(&buf, "session/update", map[string]any{
		"sessionId": "s1",
	})
	if !bytes.Contains(buf.Bytes(), []byte(`"method":"session/update"`)) {
		t.Errorf("got %s", buf.String())
	}
	if bytes.Contains(buf.Bytes(), []byte(`"id"`)) {
		t.Errorf("notifications must not carry id: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement protocol frames**

Create `acp/stdio/protocol.go`:

```go
// Package stdio implements the Agent Control Protocol over newline-
// delimited JSON-RPC 2.0 on stdin/stdout. It mirrors the protocol
// shape used by the Python acp_adapter so Zed/Cursor/VS Code clients
// work against hermind unchanged.
package stdio

import (
	"encoding/json"
	"fmt"
	"io"
)

const jsonRPCVersion = "2.0"

// Request is a decoded JSON-RPC 2.0 request frame.
type Request struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // may be absent for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification reports whether the request is a notification (no ID).
func (r *Request) IsNotification() bool {
	return len(r.ID) == 0 || string(r.ID) == "null"
}

// Response is a JSON-RPC 2.0 response frame. Exactly one of Result or
// Error is populated.
type Response struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// DecodeRequest parses a single JSON-RPC frame.
func DecodeRequest(raw []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("acp/stdio: decode: %w", err)
	}
	if req.Version != "" && req.Version != jsonRPCVersion {
		return nil, fmt.Errorf("acp/stdio: unsupported jsonrpc version %q", req.Version)
	}
	return &req, nil
}

// EncodeResponse writes a response frame followed by a single newline.
func EncodeResponse(w io.Writer, resp *Response) error {
	resp.Version = jsonRPCVersion
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

// EncodeNotification writes a server-initiated notification frame.
func EncodeNotification(w io.Writer, method string, params interface{}) error {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Version string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{jsonRPCVersion, method, rawParams})
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

Run: `go test ./acp/stdio/ -v`
Expected: PASS (4 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/protocol.go acp/stdio/protocol_test.go
git commit -m "feat(acp/stdio): JSON-RPC frame codec"
```

---

## Task 2: Session + SessionManager

**Files:**
- Create: `acp/stdio/session.go`
- Create: `acp/stdio/session_test.go`

- [ ] **Step 1: Write the failing test**

Create `acp/stdio/session_test.go`:

```go
package stdio

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func openTestStore(t *testing.T) storage.Storage {
	t.Helper()
	s, err := sqlite.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSessionManager_CreateAndGet(t *testing.T) {
	sm := NewSessionManager(openTestStore(t))
	s, err := sm.Create(context.Background(), "/tmp/work", "anthropic/claude-opus-4-6")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" {
		t.Fatal("expected session id")
	}
	got, err := sm.Get(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cwd != "/tmp/work" || got.Model != "anthropic/claude-opus-4-6" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestSessionManager_LoadMissing(t *testing.T) {
	sm := NewSessionManager(openTestStore(t))
	if _, err := sm.Get(context.Background(), "nope"); err == nil {
		t.Error("expected error for missing session")
	}
}

func TestSessionManager_AppendAndHistory(t *testing.T) {
	sm := NewSessionManager(openTestStore(t))
	s, _ := sm.Create(context.Background(), "/tmp", "m")
	if err := sm.AppendUserText(context.Background(), s.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := sm.AppendAssistantText(context.Background(), s.ID, "hi back"); err != nil {
		t.Fatal(err)
	}
	msgs, err := sm.History(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d", len(msgs))
	}
	if msgs[0].Content.Text() != "hello" || msgs[1].Content.Text() != "hi back" {
		t.Errorf("history = %+v", msgs)
	}
}
```

Note: if `storage/sqlite` does not expose a `NewMemory()` constructor, adjust the test to use whatever in-memory factory exists (e.g. `sqlite.Open(":memory:")`). Run `grep -n "func New" storage/sqlite/*.go` to find it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run TestSessionManager -v`
Expected: FAIL — `SessionManager` undefined.

- [ ] **Step 3: Implement SessionManager**

Create `acp/stdio/session.go`:

```go
package stdio

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/storage"
)

// Session is the in-memory representation of an ACP session. The
// persistent record lives in storage.Storage; this struct holds the
// runtime state (cancel function, subscribers).
type Session struct {
	ID        string
	Cwd       string
	Model     string
	CancelCtx context.CancelFunc // set while a prompt is running
}

// SessionManager owns the ACP session lifecycle. All methods are
// safe for concurrent use.
type SessionManager struct {
	mu       sync.Mutex
	store    storage.Storage
	sessions map[string]*Session
}

// NewSessionManager constructs a manager backed by the given storage.
func NewSessionManager(store storage.Storage) *SessionManager {
	return &SessionManager{
		store:    store,
		sessions: make(map[string]*Session),
	}
}

// Create allocates a new session, persists its metadata, and returns
// the runtime handle.
func (m *SessionManager) Create(ctx context.Context, cwd, model string) (*Session, error) {
	id := uuid.NewString()
	rec := &storage.Session{
		ID:     id,
		Source: "acp",
		Model:  model,
	}
	if err := m.store.CreateSession(ctx, rec); err != nil {
		return nil, fmt.Errorf("acp/stdio: create session: %w", err)
	}
	s := &Session{ID: id, Cwd: cwd, Model: model}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

// Get fetches a session. If it is not in the in-memory cache the
// manager restores it from storage.
func (m *SessionManager) Get(ctx context.Context, id string) (*Session, error) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil {
		return s, nil
	}
	rec, err := m.store.GetSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("acp/stdio: get session %s: %w", id, err)
	}
	s = &Session{ID: rec.ID, Model: rec.Model}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

// SetCancel registers the cancel function for the session's currently
// running prompt. Cancel() will invoke it.
func (m *SessionManager) SetCancel(id string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		s.CancelCtx = cancel
	}
}

// Cancel interrupts the in-flight prompt for the given session, if any.
func (m *SessionManager) Cancel(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil && s.CancelCtx != nil {
		s.CancelCtx()
	}
}

// AppendUserText adds a user-authored text message to the session history.
func (m *SessionManager) AppendUserText(ctx context.Context, id, text string) error {
	return m.store.AddMessage(ctx, id, &storage.StoredMessage{
		Role:    string(message.RoleUser),
		Content: text,
	})
}

// AppendAssistantText adds an assistant reply to the session history.
func (m *SessionManager) AppendAssistantText(ctx context.Context, id, text string) error {
	return m.store.AddMessage(ctx, id, &storage.StoredMessage{
		Role:    string(message.RoleAssistant),
		Content: text,
	})
}

// History returns the conversation history as message.Message values
// suitable for passing to provider.Provider.
func (m *SessionManager) History(ctx context.Context, id string) ([]message.Message, error) {
	stored, err := m.store.GetMessages(ctx, id, 1000, 0)
	if err != nil {
		return nil, err
	}
	out := make([]message.Message, 0, len(stored))
	for _, m := range stored {
		out = append(out, message.Message{
			Role:    message.Role(m.Role),
			Content: message.TextContent(m.Content),
		})
	}
	return out, nil
}
```

If the actual `storage.StoredMessage` field names differ, adjust: run `grep -n 'type StoredMessage' storage/types.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run TestSessionManager -v`
Expected: PASS (3 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/session.go acp/stdio/session_test.go
git commit -m "feat(acp/stdio): SessionManager backed by storage.Storage"
```

---

## Task 3: Handlers

**Files:**
- Create: `acp/stdio/handlers.go`
- Create: `acp/stdio/handlers_test.go`

- [ ] **Step 1: Write the failing test**

Create `acp/stdio/handlers_test.go`:

```go
package stdio

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage/sqlite"
)

type stubProvider struct{ reply string }

func (s *stubProvider) Name() string                                   { return "stub" }
func (s *stubProvider) Available() bool                                { return true }
func (s *stubProvider) ModelInfo(string) *provider.ModelInfo           { return &provider.ModelInfo{ContextLength: 1000, MaxOutputTokens: 100} }
func (s *stubProvider) EstimateTokens(_, t string) (int, error)        { return len(t) / 4, nil }
func (s *stubProvider) Stream(context.Context, *provider.Request) (provider.Stream, error) { return nil, nil }
func (s *stubProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return &provider.Response{
		Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent(s.reply)},
		FinishReason: "end_turn",
	}, nil
}

func newTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	store, err := sqlite.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &Handlers{
		Sessions: NewSessionManager(store),
		Factory: func(model string) (provider.Provider, error) {
			return &stubProvider{reply: "ok"}, nil
		},
		AgentCfg: config.AgentConfig{MaxTurns: 3},
	}
}

func TestHandleInitialize(t *testing.T) {
	h := newTestHandlers(t)
	raw, err := h.handleInitialize(context.Background(), json.RawMessage(`{"clientInfo":{"name":"zed"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	_ = json.Unmarshal(raw, &resp)
	if resp["protocolVersion"] == nil {
		t.Errorf("missing protocolVersion: %v", resp)
	}
	if resp["agentInfo"] == nil {
		t.Errorf("missing agentInfo: %v", resp)
	}
}

func TestHandleNewSession_ReturnsID(t *testing.T) {
	h := newTestHandlers(t)
	raw, err := h.handleNewSession(context.Background(), json.RawMessage(`{"cwd":"/tmp"}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	_ = json.Unmarshal(raw, &resp)
	if resp["sessionId"] == "" {
		t.Errorf("no sessionId: %v", resp)
	}
}

func TestHandlePrompt_EchoesProvider(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")

	params, _ := json.Marshal(map[string]any{
		"sessionId": s.ID,
		"prompt":    []any{map[string]any{"type": "text", "text": "hi"}},
	})
	raw, err := h.handlePrompt(context.Background(), params)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	var resp map[string]any
	_ = json.Unmarshal(raw, &resp)
	if resp["stopReason"] != "end_turn" {
		t.Errorf("stopReason = %v", resp["stopReason"])
	}

	// History now contains both user prompt and assistant reply.
	msgs, _ := h.Sessions.History(context.Background(), s.ID)
	if len(msgs) != 2 {
		t.Errorf("history len = %d", len(msgs))
	}
}

func TestHandleCancel_InterruptsActivePrompt(t *testing.T) {
	h := newTestHandlers(t)
	s, _ := h.Sessions.Create(context.Background(), "/tmp", "stub/model")

	ctx, cancel := context.WithCancel(context.Background())
	h.Sessions.SetCancel(s.ID, cancel)

	params, _ := json.Marshal(map[string]any{"sessionId": s.ID})
	_, err := h.handleCancel(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	// ctx was wired through cancel — after handleCancel it must be done.
	select {
	case <-ctx.Done():
		// OK
	default:
		t.Error("expected ctx done after cancel")
	}
	_ = agent.Version // silence unused import in case agent.* is only referenced elsewhere
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run TestHandle -v`
Expected: FAIL — `Handlers` undefined.

- [ ] **Step 3: Implement handlers**

Create `acp/stdio/handlers.go`:

```go
package stdio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Handlers bundles the collaborators each ACP method needs.
type Handlers struct {
	Sessions *SessionManager
	// Factory produces a provider.Provider given a model ref. The
	// caller typically passes a closure that calls factory.New with
	// the right config.ProviderConfig.
	Factory  func(model string) (provider.Provider, error)
	AgentCfg config.AgentConfig
}

// ---- initialize ----

type initializeResult struct {
	ProtocolVersion int          `json:"protocolVersion"`
	AgentInfo       agentInfo    `json:"agentInfo"`
	AgentCapability agentCap     `json:"agentCapabilities"`
	AuthMethods     []authMethod `json:"authMethods,omitempty"`
}

type agentInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type agentCap struct {
	LoadSession bool `json:"loadSession"`
}

type authMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *Handlers) handleInitialize(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	r := initializeResult{
		ProtocolVersion: 1,
		AgentInfo: agentInfo{
			Name:    "hermind",
			Version: "dev",
		},
		AgentCapability: agentCap{LoadSession: true},
	}
	return json.Marshal(r)
}

// ---- authenticate ----

// MVP does not advertise auth methods and so receives no authenticate
// call in practice. The stub returns the empty object for protocol
// compatibility with clients that send it eagerly.
func (h *Handlers) handleAuthenticate(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// ---- session/new + session/load ----

type newSessionParams struct {
	Cwd string `json:"cwd"`
	// mcpServers is accepted for protocol compatibility but ignored
	// in the MVP — MCP wiring is a Plan I concern.
	MCPServers []json.RawMessage `json:"mcpServers,omitempty"`
	Model      string            `json:"model,omitempty"`
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

func (h *Handlers) handleNewSession(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p newSessionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Cwd == "" {
		return nil, errors.New("acp/stdio: new_session: cwd is required")
	}
	model := p.Model
	if model == "" {
		model = "anthropic/claude-opus-4-6"
	}
	s, err := h.Sessions.Create(ctx, p.Cwd, model)
	if err != nil {
		return nil, err
	}
	return json.Marshal(newSessionResult{SessionID: s.ID})
}

type loadSessionParams struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd,omitempty"`
}

func (h *Handlers) handleLoadSession(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p loadSessionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	s, err := h.Sessions.Get(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	if p.Cwd != "" {
		s.Cwd = p.Cwd
	}
	return json.RawMessage(`{}`), nil
}

// ---- prompt ----

type promptContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type promptParams struct {
	SessionID string               `json:"sessionId"`
	Prompt    []promptContentBlock `json:"prompt"`
}

type promptResult struct {
	StopReason string `json:"stopReason"`
}

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

	provider, err := h.Factory(s.Model)
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

	resp, err := provider.Complete(cctx, &provider.Request{
		Model:    s.Model,
		Messages: history,
		MaxTokens: 4096,
	})
	if err != nil {
		if errors.Is(cctx.Err(), context.Canceled) {
			return json.Marshal(promptResult{StopReason: "cancelled"})
		}
		return nil, err
	}

	if err := h.Sessions.AppendAssistantText(ctx, s.ID, resp.Message.Content.Text()); err != nil {
		return nil, err
	}

	stop := resp.FinishReason
	if stop == "" {
		stop = "end_turn"
	}
	return json.Marshal(promptResult{StopReason: stop})
}

// ---- cancel ----

type cancelParams struct {
	SessionID string `json:"sessionId"`
}

func (h *Handlers) handleCancel(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p cancelParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	h.Sessions.Cancel(p.SessionID)
	return json.RawMessage(`null`), nil
}

// extractText concatenates every text block into a single string.
// Non-text blocks (image, resource_link) are ignored in the MVP.
func extractText(blocks []promptContentBlock) string {
	var total string
	for _, b := range blocks {
		if b.Type == "text" {
			total += b.Text
		}
	}
	return total
}

// Unused import silencer; remove when agent package is actually used.
var _ = message.RoleUser
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run TestHandle -v`
Expected: PASS (4 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/handlers.go acp/stdio/handlers_test.go
git commit -m "feat(acp/stdio): initialize, new_session, prompt, cancel handlers"
```

---

## Task 4: Read-loop server

**Files:**
- Create: `acp/stdio/server.go`
- Create: `acp/stdio/server_test.go`

- [ ] **Step 1: Write the failing test**

Create `acp/stdio/server_test.go`:

```go
package stdio

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestServer_EndToEnd_NewSessionAndPrompt(t *testing.T) {
	h := newTestHandlers(t)

	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/tmp"}}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"sessionId":"__fill__","prompt":[{"type":"text","text":"hi"}]}}` + "\n")
	// Replace __fill__ by intercepting after session/new — simplest approach
	// is a two-phase test. Split the input so we can stitch in the returned ID.

	var out bytes.Buffer
	srv := NewServer(h)

	// Phase 1: drive initialize + session/new.
	phase1 := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/tmp"}}` + "\n")
	if err := srv.RunOnce(context.Background(), phase1, &out, 2); err != nil {
		t.Fatalf("phase1: %v", err)
	}
	// Extract session id from the second response line.
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), out.String())
	}
	var resp2 struct {
		Result newSessionResult `json:"result"`
	}
	_ = json.Unmarshal([]byte(lines[1]), &resp2)
	sessionID := resp2.Result.SessionID
	if sessionID == "" {
		t.Fatalf("no session id in %q", lines[1])
	}

	// Phase 2: drive prompt.
	out.Reset()
	phase2Input := `{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"sessionId":"` + sessionID + `","prompt":[{"type":"text","text":"hi"}]}}` + "\n"
	if err := srv.RunOnce(context.Background(), strings.NewReader(phase2Input), &out, 1); err != nil {
		t.Fatalf("phase2: %v", err)
	}
	if !strings.Contains(out.String(), `"stopReason":"end_turn"`) {
		t.Errorf("unexpected response: %s", out.String())
	}

	_ = in       // silence unused variable warning
	_ = time.Now // silence unused import
}

func TestServer_UnknownMethodReturnsError(t *testing.T) {
	h := newTestHandlers(t)
	srv := NewServer(h)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"does_not_exist"}` + "\n")
	var out bytes.Buffer
	_ = srv.RunOnce(context.Background(), in, &out, 1)
	if !strings.Contains(out.String(), `"code":-32601`) {
		t.Errorf("expected method-not-found error, got %s", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./acp/stdio/ -run TestServer -v`
Expected: FAIL — `Server` undefined.

- [ ] **Step 3: Implement the server**

Create `acp/stdio/server.go`:

```go
package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Server drives the stdio read/dispatch loop.
type Server struct {
	handlers *Handlers

	// writeMu serializes stdout writes so concurrent prompt handlers
	// and notification senders don't interleave JSON frames.
	writeMu sync.Mutex
}

// NewServer constructs a server with the given handler bundle.
func NewServer(h *Handlers) *Server {
	return &Server{handlers: h}
}

// Run reads frames from r and writes responses to w until r hits EOF
// or ctx is cancelled. Non-fatal decode errors are reported as
// parse-error responses; they don't abort the loop.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	return s.RunOnce(ctx, r, w, -1)
}

// RunOnce is a test seam that stops after reading n frames (pass -1 for
// "until EOF").
func (s *Server) RunOnce(ctx context.Context, r io.Reader, w io.Writer, n int) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<16), 1<<22) // up to 4 MiB per frame
	count := 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		s.dispatch(ctx, append([]byte{}, line...), w)
		count++
		if n > 0 && count >= n {
			return nil
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, raw []byte, w io.Writer) {
	req, err := DecodeRequest(raw)
	if err != nil {
		s.write(w, &Response{
			Error: &Error{Code: CodeParseError, Message: err.Error()},
		})
		return
	}

	result, err := s.route(ctx, req.Method, req.Params)
	if req.IsNotification() {
		// Notifications take no response. Dispatch still runs for side effects.
		return
	}
	resp := &Response{ID: req.ID}
	if err != nil {
		resp.Error = &Error{Code: CodeInternalError, Message: err.Error()}
	} else {
		resp.Result = result
	}
	s.write(w, resp)
}

// route dispatches to the right handler by method name. Method names
// mirror the Python acp library (snake_case) and the Zed ACP spec
// (slash-separated). We accept both shapes.
func (s *Server) route(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "initialize":
		return s.handlers.handleInitialize(ctx, params)
	case "authenticate":
		return s.handlers.handleAuthenticate(ctx, params)
	case "session/new", "newSession", "new_session":
		return s.handlers.handleNewSession(ctx, params)
	case "session/load", "loadSession", "load_session":
		return s.handlers.handleLoadSession(ctx, params)
	case "session/prompt", "prompt":
		return s.handlers.handlePrompt(ctx, params)
	case "session/cancel", "cancel":
		return s.handlers.handleCancel(ctx, params)
	}
	return nil, &routingError{method: method}
}

type routingError struct{ method string }

func (e *routingError) Error() string { return fmt.Sprintf("method not found: %s", e.method) }

// Override error code for unknown methods so clients see -32601.
func (s *Server) write(w io.Writer, resp *Response) {
	if resp.Error != nil {
		if _, ok := isRouteError(resp); ok {
			resp.Error.Code = CodeMethodNotFound
		}
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = EncodeResponse(w, resp)
}

func isRouteError(r *Response) (*routingError, bool) {
	// Wrapped error crosses the route() → dispatch() boundary as a
	// plain error via Error.Message. The original *routingError does
	// not survive the round-trip, so we fall back to prefix matching.
	// This is best-effort — the method-not-found code only sets when
	// the message matches "method not found: ...".
	if r.Error == nil {
		return nil, false
	}
	const prefix = "method not found: "
	if len(r.Error.Message) >= len(prefix) && r.Error.Message[:len(prefix)] == prefix {
		return &routingError{method: r.Error.Message[len(prefix):]}, true
	}
	return nil, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./acp/stdio/ -run TestServer -v -race`
Expected: PASS (2 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add acp/stdio/server.go acp/stdio/server_test.go
git commit -m "feat(acp/stdio): read/dispatch loop with method routing"
```

---

## Task 5: CLI wiring

**Files:**
- Create: `cli/acp.go`
- Create: `cli/acp_test.go`
- Modify: `cli/root.go`

- [ ] **Step 1: Write the failing test**

Create `cli/acp_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestACPCmd_InitializeRoundTrip(t *testing.T) {
	app := &App{}

	cmd := newACPCmd(app)
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"

	cmd.SetIn(strings.NewReader(input))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), `"protocolVersion"`) {
		t.Errorf("missing protocolVersion: %s", stdout.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestACPCmd -v`
Expected: FAIL — `newACPCmd` undefined.

- [ ] **Step 3: Implement the CLI command**

Create `cli/acp.go`:

```go
package cli

import (
	"github.com/odysseythink/hermind/acp/stdio"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/spf13/cobra"
)

// newACPCmd returns the "hermind acp" subcommand. It reads JSON-RPC
// frames from stdin and writes responses to stdout; logs are emitted
// to stderr (handled elsewhere by the logging package).
func newACPCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "acp",
		Short: "Run hermind as an ACP stdio server (editor integration)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app != nil {
				if err := ensureStorage(app); err != nil {
					return err
				}
			}

			var store interface {
				stdio.SessionStorage
			}
			if app != nil && app.Storage != nil {
				store = app.Storage
			} else {
				// Fallback: in-memory store so `hermind acp` can boot even
				// before first-run configuration completes.
				return cmd.Help()
			}

			handlers := &stdio.Handlers{
				Sessions: stdio.NewSessionManager(app.Storage),
				Factory: func(model string) (provider.Provider, error) {
					cfg := resolveProviderConfig(app.Config, model)
					return factory.New(cfg)
				},
				AgentCfg: app.Config.Agent,
			}
			srv := stdio.NewServer(handlers)
			return srv.Run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

// resolveProviderConfig maps a "<name>/<model>" ref to the ProviderConfig
// from app.Config.Providers, overriding the model.
func resolveProviderConfig(cfg *config.Config, modelRef string) config.ProviderConfig {
	name, model := splitModelRef(modelRef)
	p := cfg.Providers[name]
	if model != "" {
		p.Model = model
	}
	return p
}
```

If `splitModelRef` is already defined by Plan C's `cli/batch.go`, do not redeclare it. If Plan C has not landed yet, add this helper temporarily:

```go
func splitModelRef(ref string) (string, string) {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '/' {
			return ref[:i], ref[i+1:]
		}
	}
	return ref, ""
}
```

Also: delete the unused `store` / `SessionStorage` block from the body — the imported `app.Storage` is already a `storage.Storage` that satisfies whatever `NewSessionManager` needs. Final body:

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureStorage(app); err != nil {
				return err
			}
			handlers := &stdio.Handlers{
				Sessions: stdio.NewSessionManager(app.Storage),
				Factory: func(model string) (provider.Provider, error) {
					cfg := resolveProviderConfig(app.Config, model)
					return factory.New(cfg)
				},
				AgentCfg: app.Config.Agent,
			}
			srv := stdio.NewServer(handlers)
			return srv.Run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
```

Note: because the test creates an `App{}` with no storage, the test needs either an in-memory `app.Storage` or a handler that does not touch storage. The simplest fix is to seed the test with an in-memory store:

Update `TestACPCmd_InitializeRoundTrip`:

```go
func TestACPCmd_InitializeRoundTrip(t *testing.T) {
	store, err := sqlite.NewMemory() // import storage/sqlite
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	app := &App{Config: &config.Config{}, Storage: store}
	// ... rest unchanged
}
```

- [ ] **Step 4: Register in root**

In `cli/root.go`, add `newACPCmd(app),` to the `AddCommand(...)` call.

- [ ] **Step 5: Run tests**

Run: `go test ./cli/ -run TestACPCmd -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add cli/acp.go cli/acp_test.go cli/root.go
git commit -m "feat(cli): add 'hermind acp' stdio subcommand"
```

---

## Task 6: Manual smoke test

- [ ] **Step 1: Build**

```bash
go build -o /tmp/hermind ./cmd/hermind
```

- [ ] **Step 2: Drive from the shell**

```bash
printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/tmp"}}' \
  | /tmp/hermind acp 2>/tmp/hermind-acp.log
```

Expected: two newline-separated JSON frames on stdout — initialize result then the new session ID. `/tmp/hermind-acp.log` contains internal logs (not stdout).

- [ ] **Step 3: Cleanup**

```bash
rm /tmp/hermind /tmp/hermind-acp.log
```

- [ ] **Step 4: Optional marker commit**

```bash
git commit --allow-empty -m "test(acp/stdio): manual stdin/stdout round-trip verified"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - JSON-RPC frame codec ↔ Task 1 ✓
   - SessionManager + storage.Storage persistence ↔ Task 2 ✓
   - initialize / authenticate / new_session / load_session / prompt / cancel ↔ Task 3 ✓
   - Read/dispatch loop with method routing ↔ Task 4 ✓
   - CLI wiring ↔ Task 5 ✓
   - Unknown method returns JSON-RPC -32601 ↔ Task 4 ✓

2. **Placeholders:** Task 5 Step 3 includes a deliberate cleanup path (removing the `store` / `SessionStorage` block and switching to `ensureStorage(app)`). Code is provided for both phases.

3. **Type consistency:**
   - `Handlers{Sessions, Factory, AgentCfg}` shape stable in Tasks 3, 4, 5.
   - `Server.RunOnce(ctx, r, w, n)` stable Task 4 + server test.
   - `Session.ID, Cwd, Model, CancelCtx` stable Tasks 2, 3.
   - Method names accepted in both snake_case and slash/camelCase forms in Task 4 for maximum client compatibility.

4. **Gaps (deferred to Plan F):**
   - `session/update` notifications (agent_message_chunk, tool_call_start/update, available_commands_update).
   - `requestPermission` round-trip for terminal/destructive tools.
   - `authMethods` population from detected providers.
   - `.well-known/agent.json` registry emission.
   - `fork_session`, `list_sessions`, `set_session_model`, `set_session_mode`.

---

## Definition of Done

- `go test ./acp/stdio/... ./cli/... -race` all pass.
- `go build ./...` succeeds.
- Manual smoke test (`initialize` + `session/new` round-trip over stdin/stdout) succeeds.
- Sessions are persisted in `storage.Storage` with `Source="acp"` and survive a second-process `session/load`.
