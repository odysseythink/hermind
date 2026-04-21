# Web Chat Backend (Phase 1/3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `POST /api/sessions/{id}/messages` (submit a user message, Engine runs in a goroutine, events stream via existing SSE/WS) and `POST /api/sessions/{id}/cancel` (ctx-cancel the running engine). Extract a shared `BuildEngineDeps` for both TUI and web.

**Architecture:** Per-request Engine driven by a new `api/sessionrun` package. In-memory `SessionRegistry` (`map[string]context.CancelFunc + Mutex`) tracks running sessions for busy-rejection and cancellation. Handlers return 202 immediately and spawn a goroutine that publishes events to the existing `StreamHub`.

**Tech Stack:** Go, go-chi router, existing `agent.Engine`, existing `api.StreamHub`, sqlite storage.

**Spec:** `docs/superpowers/specs/2026-04-21-web-chat-backend-design.md`

---

## File map

| File | Action | Purpose |
|---|---|---|
| `api/session_registry.go` | Create | `SessionRegistry` with Register/Cancel/IsBusy/Clear |
| `api/session_registry_test.go` | Create | Unit tests |
| `api/sessionrun/runner.go` | Create | `Deps`, `Request`, `Run(ctx, Deps, Request) error` |
| `api/sessionrun/runner_test.go` | Create | Unit tests with fake provider + hub |
| `api/handlers_session_run.go` | Create | `handleSessionMessagesPost`, `handleSessionCancel` |
| `api/handlers_session_run_test.go` | Create | End-to-end via httptest + msw-style fakes |
| `api/dto.go` | Modify | Add `MessageSubmitRequest` / `MessageSubmitResponse` |
| `api/server.go` | Modify | `ServerOpts` gains `Deps sessionrun.Deps` + `Registry *SessionRegistry`; register two routes |
| `cli/engine_deps.go` | Create | `BuildEngineDeps(cfg *config.Config) (sessionrun.Deps, error)` extracted from `cli/repl.go` |
| `cli/engine_deps_test.go` | Create | Unit tests for the builder |
| `cli/web.go` | Modify | Call `BuildEngineDeps`; pass to `ServerOpts` |
| `cli/repl.go` | Modify | Call `BuildEngineDeps`; delete the inlined construction block (lines ~44-145) |

---

### Task 1: SessionRegistry

Create the smallest in-memory registry for tracking running sessions.

**Files:**
- Create: `api/session_registry.go`
- Create: `api/session_registry_test.go`

- [ ] **Step 1: Write failing tests**

Create `api/session_registry_test.go`:

```go
package api

import (
	"context"
	"sync"
	"testing"
)

func TestRegistry_RegisterAndCancel(t *testing.T) {
	r := NewSessionRegistry()
	called := false
	ok := r.Register("s1", func() { called = true })
	if !ok {
		t.Fatal("Register should return true on first insert")
	}
	if !r.IsBusy("s1") {
		t.Error("IsBusy should be true after Register")
	}
	cancelled := r.Cancel("s1")
	if !cancelled {
		t.Error("Cancel should return true for known id")
	}
	if !called {
		t.Error("Cancel should invoke the stored func")
	}
	if r.IsBusy("s1") {
		t.Error("IsBusy should be false after Cancel")
	}
	if r.Cancel("s1") {
		t.Error("second Cancel should return false")
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewSessionRegistry()
	r.Register("s1", func() {})
	if r.Register("s1", func() {}) {
		t.Error("second Register for same id should return false")
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := NewSessionRegistry()
	r.Register("s1", func() {})
	r.Clear("s1")
	if r.IsBusy("s1") {
		t.Error("IsBusy should be false after Clear")
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := NewSessionRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + (i % 26))) + "-" + string(rune('0'+(i%10)))
			r.Register(id, func() {})
			r.Clear(id)
		}(i)
	}
	wg.Wait()
	_ = context.Background()
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./api -run TestRegistry -v
```

Expected: compile errors — `undefined: NewSessionRegistry`, etc.

- [ ] **Step 3: Implement**

Create `api/session_registry.go`:

```go
package api

import "sync"

// SessionRegistry tracks which session IDs currently have a running
// Engine invocation, along with the cancel function that stops it.
// Zero value is not usable; use NewSessionRegistry.
type SessionRegistry struct {
	mu      sync.Mutex
	running map[string]func()
}

// NewSessionRegistry returns an empty registry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{running: make(map[string]func())}
}

// Register stores cancel under id. Returns false if id is already present,
// in which case the caller is expected to reject the request (busy).
func (r *SessionRegistry) Register(id string, cancel func()) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.running[id]; ok {
		return false
	}
	r.running[id] = cancel
	return true
}

// Cancel invokes and removes the cancel func for id. Returns false if
// the id was not registered (e.g., already cleared, or never running).
func (r *SessionRegistry) Cancel(id string) bool {
	r.mu.Lock()
	cancel, ok := r.running[id]
	if ok {
		delete(r.running, id)
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// Clear removes id without invoking its cancel func. Used by the
// runner goroutine on natural completion.
func (r *SessionRegistry) Clear(id string) {
	r.mu.Lock()
	delete(r.running, id)
	r.mu.Unlock()
}

// IsBusy reports whether id is currently registered.
func (r *SessionRegistry) IsBusy(id string) bool {
	r.mu.Lock()
	_, ok := r.running[id]
	r.mu.Unlock()
	return ok
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test -race ./api -run TestRegistry -v
```

Expected: PASS. Race detector clean on TestRegistry_Concurrent.

- [ ] **Step 5: Commit**

```bash
git add api/session_registry.go api/session_registry_test.go
git commit -m "feat(api): SessionRegistry tracks running engine per session"
```

---

### Task 2: sessionrun package skeleton

Define `Deps`, `Request`, `Run` signature. No logic yet.

**Files:**
- Create: `api/sessionrun/runner.go`

- [ ] **Step 1: Create the skeleton**

```go
// Package sessionrun runs one agent.Engine invocation per HTTP request
// and publishes its streaming events to an api.StreamHub.
//
// The package sits between the HTTP layer (api/handlers_session_run.go)
// and the Engine; it exists as its own package so tests can drive it
// without spinning up a Server, and so a future CLI entry (or the
// current TUI, before Phase 3) can reuse it.
package sessionrun

import (
	"context"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// EventPublisher is the minimal interface Run needs from api.StreamHub.
// Keeping it as an interface avoids an import cycle (api/sessionrun must
// not import api).
type EventPublisher interface {
	Publish(event Event)
}

// Event mirrors api.StreamEvent. Kept separate to avoid the cycle; the
// api package adapts between the two shapes.
type Event struct {
	Type      string
	SessionID string
	Data      any
}

// Deps bundles everything Run needs to build an Engine.
type Deps struct {
	Provider    provider.Provider
	AuxProvider provider.Provider // optional
	Storage     storage.Storage
	ToolReg     *tool.Registry
	SkillsReg   *skills.Registry // optional
	AgentCfg    config.AgentConfig
	Hub         EventPublisher
}

// Request is one user message submission.
type Request struct {
	SessionID   string
	UserMessage string
	Model       string
}

// Run builds an Engine, wires stream callbacks into Hub, and invokes
// RunConversation. It publishes status and token events through Hub.
// Returns the Engine's error verbatim (including context.Canceled).
// On panic, recovers and publishes a status(error) event.
func Run(ctx context.Context, deps Deps, req Request) (err error) {
	// Implementation in Task 3.
	return nil
}

// ActiveSkillsBridge converts a skills.Registry into the
// agent.ActiveSkill shape the Engine expects.
func ActiveSkillsBridge(reg *skills.Registry) func() []agent.ActiveSkill {
	return func() []agent.ActiveSkill {
		active := reg.Active()
		out := make([]agent.ActiveSkill, 0, len(active))
		for _, s := range active {
			out = append(out, agent.ActiveSkill{
				Name:        s.Name,
				Description: s.Description,
				Body:        s.Body,
			})
		}
		return out
	}
}
```

- [ ] **Step 2: Compile**

```bash
go build ./api/sessionrun
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add api/sessionrun/runner.go
git commit -m "feat(api/sessionrun): skeleton types (Deps, Request, Run stub)"
```

---

### Task 3: sessionrun.Run — happy path

Implement Run for the no-tool, no-error case. Cover with a test using a fake provider + fake hub.

**Files:**
- Modify: `api/sessionrun/runner.go` (Run body)
- Create: `api/sessionrun/runner_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/sessionrun/runner_test.go`:

```go
package sessionrun

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// --- fake hub ---

type fakeHub struct {
	mu     sync.Mutex
	events []Event
}

func (h *fakeHub) Publish(e Event) {
	h.mu.Lock()
	h.events = append(h.events, e)
	h.mu.Unlock()
}

func (h *fakeHub) types() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := make([]string, len(h.events))
	for i, e := range h.events {
		t[i] = e.Type
	}
	return t
}

// --- fake provider that emits deltas then completes ---

type fakeProvider struct {
	deltas []string
	err    error
}

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) Complete(ctx context.Context, req *provider.CompleteRequest) (*message.Message, error) {
	panic("Complete unused in this test — Stream is the path")
}
func (p *fakeProvider) Stream(ctx context.Context, req *provider.CompleteRequest, cb func(*provider.StreamDelta)) (*message.Message, error) {
	for _, d := range p.deltas {
		if p.err != nil {
			return nil, p.err
		}
		cb(&provider.StreamDelta{ContentDelta: d})
	}
	return &message.Message{Role: message.RoleAssistant, Content: message.NewTextContent(strings.Join(p.deltas, ""))}, nil
}

// --- test ---

func TestRun_HappyPath(t *testing.T) {
	hub := &fakeHub{}
	store := storage.NewMemoryStorage() // or storage/memory; see note below
	deps := Deps{
		Provider: &fakeProvider{deltas: []string{"Hello, ", "world"}},
		Storage:  store,
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{},
		Hub:      hub,
	}
	err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "hi"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	types := hub.types()
	// Expected sequence: status(running), token, token, message_complete, status(idle)
	want := []string{"status", "token", "token", "message_complete", "status"}
	if len(types) != len(want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("events[%d] = %q, want %q", i, types[i], w)
		}
	}
}
```

> **If** `storage.NewMemoryStorage()` does not exist, use `sqlite.New(":memory:")` or whichever in-memory storage the repo already has. Adjust the import accordingly. If nothing exists, add a minimal in-memory fake in the test file (not in the main package).

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./api/sessionrun -run TestRun_HappyPath -v
```

Expected: FAIL — Run returns nil, no events published.

- [ ] **Step 3: Implement Run**

Replace the Run stub in `api/sessionrun/runner.go`:

```go
func Run(ctx context.Context, deps Deps, req Request) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			deps.Hub.Publish(Event{
				Type:      "status",
				SessionID: req.SessionID,
				Data:      map[string]any{"state": "error", "error": "internal: " + toError(rec).Error()},
			})
			err = toError(rec)
		}
	}()

	engine := agent.NewEngineWithToolsAndAux(
		deps.Provider, deps.AuxProvider, deps.Storage, deps.ToolReg,
		deps.AgentCfg, "web",
	)
	if deps.SkillsReg != nil {
		engine.SetActiveSkillsProvider(ActiveSkillsBridge(deps.SkillsReg))
	}
	engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
		deps.Hub.Publish(Event{
			Type:      "token",
			SessionID: req.SessionID,
			Data:      map[string]any{"text": d.ContentDelta},
		})
	})
	engine.SetToolStartCallback(func(call message.ContentBlock) {
		deps.Hub.Publish(Event{
			Type:      "tool_call",
			SessionID: req.SessionID,
			Data:      call,
		})
	})
	engine.SetToolResultCallback(func(call message.ContentBlock, result string) {
		deps.Hub.Publish(Event{
			Type:      "tool_result",
			SessionID: req.SessionID,
			Data:      map[string]any{"call": call, "result": result},
		})
	})

	deps.Hub.Publish(Event{
		Type:      "status",
		SessionID: req.SessionID,
		Data:      map[string]any{"state": "running"},
	})

	result, err := engine.RunConversation(ctx, &agent.RunOptions{
		UserMessage: req.UserMessage,
		SessionID:   req.SessionID,
		Model:       req.Model,
	})

	switch {
	case errors.Is(err, context.Canceled):
		deps.Hub.Publish(Event{
			Type:      "status",
			SessionID: req.SessionID,
			Data:      map[string]any{"state": "cancelled"},
		})
		return err
	case err != nil:
		deps.Hub.Publish(Event{
			Type:      "status",
			SessionID: req.SessionID,
			Data:      map[string]any{"state": "error", "error": err.Error()},
		})
		return err
	default:
		assistantText := ""
		if result != nil && result.Response.Content.IsText() {
			assistantText = result.Response.Content.Text()
		}
		deps.Hub.Publish(Event{
			Type:      "message_complete",
			SessionID: req.SessionID,
			Data:      map[string]any{"assistant_text": assistantText},
		})
		deps.Hub.Publish(Event{
			Type:      "status",
			SessionID: req.SessionID,
			Data:      map[string]any{"state": "idle"},
		})
		return nil
	}
}

func toError(rec any) error {
	if e, ok := rec.(error); ok {
		return e
	}
	return fmt.Errorf("%v", rec)
}
```

Add these imports to `api/sessionrun/runner.go`:

```go
import (
	"context"
	"errors"
	"fmt"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)
```

> **Verify content API:** If `message.Content` does not have `IsText()` / `Text()` methods with those exact names, grep `message/content.go` for equivalent accessors and substitute. Do not invent.

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./api/sessionrun -run TestRun_HappyPath -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/sessionrun/runner.go api/sessionrun/runner_test.go
git commit -m "feat(api/sessionrun): Run publishes status+tokens for happy path"
```

---

### Task 4: sessionrun.Run — tool call path

**Files:**
- Modify: `api/sessionrun/runner_test.go` (new test)

- [ ] **Step 1: Write the test**

Append to `api/sessionrun/runner_test.go`:

```go
func TestRun_ToolCall(t *testing.T) {
	hub := &fakeHub{}
	store := storage.NewMemoryStorage() // adjust if memory storage is elsewhere

	reg := tool.NewRegistry()
	reg.Register(&tool.Definition{
		Name:        "echo",
		Description: "echo input back",
		Run: func(ctx context.Context, args tool.Args) (string, error) {
			return "echoed: " + args.Raw(), nil
		},
	})

	// Fake provider that asks for one tool call then returns final text
	p := &toolCallingProvider{toolName: "echo", args: `{"x":1}`}

	deps := Deps{
		Provider: p,
		Storage:  store,
		ToolReg:  reg,
		AgentCfg: config.AgentConfig{},
		Hub:      hub,
	}
	if err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "go"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	types := hub.types()
	sawCall, sawResult := false, false
	for _, ty := range types {
		if ty == "tool_call" {
			sawCall = true
		}
		if ty == "tool_result" {
			sawResult = true
		}
	}
	if !sawCall || !sawResult {
		t.Errorf("events missing tool_call/tool_result: %v", types)
	}
}

type toolCallingProvider struct {
	toolName string
	args     string
	turn     int
}

func (p *toolCallingProvider) Name() string { return "fake" }
func (p *toolCallingProvider) Complete(ctx context.Context, req *provider.CompleteRequest) (*message.Message, error) {
	panic("unused")
}
func (p *toolCallingProvider) Stream(ctx context.Context, req *provider.CompleteRequest, cb func(*provider.StreamDelta)) (*message.Message, error) {
	p.turn++
	if p.turn == 1 {
		return &message.Message{
			Role:      message.RoleAssistant,
			Content:   message.NewTextContent(""),
			ToolCalls: []message.ToolCall{{
				ID:   "tc-1",
				Type: "function",
				Function: message.ToolCallFunction{Name: p.toolName, Arguments: p.args},
			}},
		}, nil
	}
	cb(&provider.StreamDelta{ContentDelta: "done"})
	return &message.Message{Role: message.RoleAssistant, Content: message.NewTextContent("done")}, nil
}
```

> **Verify:** tool.Definition fields / tool.Args API / message.NewTextContent — substitute to the real API names by grepping. If RunConversation drives tool execution differently, the test provider shape will need adjusting.

- [ ] **Step 2: Run**

```bash
go test ./api/sessionrun -run TestRun_ToolCall -v
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add api/sessionrun/runner_test.go
git commit -m "test(api/sessionrun): Run publishes tool_call/tool_result events"
```

---

### Task 5: sessionrun.Run — error / cancel / panic

Three tests, all against the same code path.

**Files:**
- Modify: `api/sessionrun/runner_test.go`

- [ ] **Step 1: Add three tests**

Append:

```go
func TestRun_ProviderError(t *testing.T) {
	hub := &fakeHub{}
	store := storage.NewMemoryStorage()
	deps := Deps{
		Provider: &fakeProvider{err: errors.New("provider down")},
		Storage:  store,
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{},
		Hub:      hub,
	}
	err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "hi"})
	if err == nil || !strings.Contains(err.Error(), "provider down") {
		t.Fatalf("expected provider error, got %v", err)
	}
	types := hub.types()
	if len(types) == 0 || types[len(types)-1] != "status" {
		t.Errorf("last event should be status(error): %v", types)
	}
}

func TestRun_ContextCancelled(t *testing.T) {
	hub := &fakeHub{}
	store := storage.NewMemoryStorage()
	// Provider that blocks until ctx done
	block := make(chan struct{})
	p := &blockingProvider{block: block}
	deps := Deps{
		Provider: p, Storage: store, ToolReg: tool.NewRegistry(),
		AgentCfg: config.AgentConfig{}, Hub: hub,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, deps, Request{SessionID: "s1", UserMessage: "hi"}) }()
	cancel()
	close(block)

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	// Last event should be status{state:cancelled}
	types := hub.types()
	if len(types) == 0 || types[len(types)-1] != "status" {
		t.Errorf("last event should be status(cancelled): %v", types)
	}
}

type blockingProvider struct{ block chan struct{} }

func (p *blockingProvider) Name() string { return "block" }
func (p *blockingProvider) Complete(ctx context.Context, req *provider.CompleteRequest) (*message.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.block:
		return nil, ctx.Err() // always return ctx err — keeps the test predictable
	}
}
func (p *blockingProvider) Stream(ctx context.Context, req *provider.CompleteRequest, cb func(*provider.StreamDelta)) (*message.Message, error) {
	return p.Complete(ctx, req)
}

func TestRun_PanicRecovered(t *testing.T) {
	hub := &fakeHub{}
	store := storage.NewMemoryStorage()
	deps := Deps{
		Provider: &panicProvider{}, Storage: store, ToolReg: tool.NewRegistry(),
		AgentCfg: config.AgentConfig{}, Hub: hub,
	}
	err := Run(context.Background(), deps, Request{SessionID: "s1", UserMessage: "hi"})
	if err == nil {
		t.Fatal("panic should surface as error")
	}
	types := hub.types()
	if len(types) == 0 || types[len(types)-1] != "status" {
		t.Errorf("last event should be status(error): %v", types)
	}
}

type panicProvider struct{}

func (p *panicProvider) Name() string { return "panic" }
func (p *panicProvider) Complete(ctx context.Context, req *provider.CompleteRequest) (*message.Message, error) {
	panic("boom")
}
func (p *panicProvider) Stream(ctx context.Context, req *provider.CompleteRequest, cb func(*provider.StreamDelta)) (*message.Message, error) {
	panic("boom")
}
```

Add `"time"` to the test file imports.

- [ ] **Step 2: Run**

```bash
go test ./api/sessionrun -v
```

Expected: all 4 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add api/sessionrun/runner_test.go
git commit -m "test(api/sessionrun): error/cancel/panic paths"
```

---

### Task 6: Extract BuildEngineDeps

Lift `cli/repl.go:44-145` (provider + aux + tool registry + skills construction) into a new shared helper so both TUI and web wire Deps the same way.

**Files:**
- Create: `cli/engine_deps.go`
- Create: `cli/engine_deps_test.go`
- Modify: `cli/repl.go` (remove inlined block, call helper)

- [ ] **Step 1: Read the existing block**

Open `cli/repl.go`, identify the sequence that builds `primaryProvider`, `providers`, fallback chain, `auxProvider`, `toolRegistry` (all built-in registrations), and skills. This block starts around line 44 and ends around line 145. Copy it verbatim into a scratch file for reference.

- [ ] **Step 2: Create the helper**

Create `cli/engine_deps.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/odysseythink/hermind/api/sessionrun"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/browser"
	"github.com/odysseythink/hermind/tool/delegate"
	"github.com/odysseythink/hermind/tool/file"
	"github.com/odysseythink/hermind/tool/mcp"
	"github.com/odysseythink/hermind/tool/memory"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/hermind/tool/terminal"
	"github.com/odysseythink/hermind/tool/vision"
	"github.com/odysseythink/hermind/tool/web"
)

// BuildEngineDeps assembles the shared provider/tool/skills dependency
// bundle. Called by both the TUI entry (cli/repl.go) and the web entry
// (cli/web.go). Hub is not filled here — callers attach it.
func BuildEngineDeps(cfg *config.Config, store storage.Storage) (sessionrun.Deps, error) {
	primary, primaryName, err := buildPrimaryProvider(cfg)
	if err != nil {
		return sessionrun.Deps{}, err
	}
	_ = primaryName

	providers := []provider.Provider{primary}
	for i, fb := range cfg.FallbackProviders {
		if fb.APIKey == "" {
			fmt.Fprintf(os.Stderr, "hermind: warning: fallback_providers[%d] (%s) has no api_key — skipping\n", i, fb.Provider)
			continue
		}
		p, err := factory.New(fb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hermind: warning: fallback_providers[%d] (%s): %v — skipping\n", i, fb.Provider, err)
			continue
		}
		providers = append(providers, p)
	}
	var p provider.Provider
	if len(providers) == 1 {
		p = providers[0]
	} else {
		p = provider.NewFallbackChain(providers)
	}

	var aux provider.Provider
	if cfg.Auxiliary.APIKey != "" || cfg.Auxiliary.Provider != "" {
		auxCfg := config.ProviderConfig{
			Provider: cfg.Auxiliary.Provider,
			BaseURL:  cfg.Auxiliary.BaseURL,
			APIKey:   cfg.Auxiliary.APIKey,
			Model:    cfg.Auxiliary.Model,
		}
		if auxCfg.Provider == "" {
			auxCfg.Provider = "anthropic"
		}
		if ap, err := factory.New(auxCfg); err == nil {
			aux = ap
		}
	}

	toolReg := tool.NewRegistry()
	file.RegisterAll(toolReg)
	termCfg := terminal.Config{
		Cwd:              cfg.Terminal.Cwd,
		DockerImage:      cfg.Terminal.DockerImage,
		DockerVolumes:    cfg.Terminal.DockerVolumes,
		SSHHost:          cfg.Terminal.SSHHost,
		SSHUser:          cfg.Terminal.SSHUser,
		SSHKey:           cfg.Terminal.SSHKey,
		SingularityImage: cfg.Terminal.SingularityImage,
		ModalBaseURL:     cfg.Terminal.ModalBaseURL,
		ModalToken:       cfg.Terminal.ModalToken,
		DaytonaBaseURL:   cfg.Terminal.DaytonaBaseURL,
		DaytonaToken:     cfg.Terminal.DaytonaToken,
	}
	if cfg.Terminal.Timeout > 0 {
		termCfg.Timeout = time.Duration(cfg.Terminal.Timeout) * time.Second
	}
	backend, err := terminal.New(cfg.Terminal.Backend, termCfg)
	if err != nil {
		return sessionrun.Deps{}, fmt.Errorf("terminal init: %w", err)
	}
	terminal.Register(toolReg, backend)
	web.RegisterAll(toolReg)
	vision.Register(toolReg, aux)
	browser.Register(toolReg, cfg.Browser)
	memory.Register(toolReg, memprovider.New(store))
	mcp.Register(toolReg, cfg.MCP)
	delegate.Register(toolReg, p, aux, store, cfg.Agent)

	var skillsReg *skills.Registry
	if s := loadSkillsIfConfigured(cfg); s != nil {
		skillsReg = s
	}

	return sessionrun.Deps{
		Provider:    p,
		AuxProvider: aux,
		Storage:     store,
		ToolReg:     toolReg,
		SkillsReg:   skillsReg,
		AgentCfg:    cfg.Agent,
	}, nil
}

// loadSkillsIfConfigured mirrors the TUI's existing lookup. Extract the
// equivalent of cli/repl.go's skills init into here; if repl.go today
// just calls skills.Load or similar, copy that call verbatim.
func loadSkillsIfConfigured(cfg *config.Config) *skills.Registry {
	reg, err := skills.Load(cfg.SkillsDir())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "hermind: skills load: %v\n", err)
		}
		return nil
	}
	return reg
}
```

> **Verify**: the exact tool registrations / skills loader call in `cli/repl.go` may differ slightly from the above. Read `cli/repl.go:44-145` and copy the current code verbatim, substituting `store` for `app.Storage` and `cfg` for `app.Config`. The list above is a reference, not a mandate.

- [ ] **Step 3: Shorten repl.go to use BuildEngineDeps**

In `cli/repl.go`, replace the block from "Open storage lazily" through the end of tool/skills construction (~lines 36-160, inclusive of `delegate.Register(...)` and skills loading) with:

```go
	if err := ensureStorage(app); err != nil {
		return err
	}
	deps, err := BuildEngineDeps(app.Config, app.Storage)
	if err != nil {
		if errors.Is(err, errMissingAPIKey) {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			fmt.Fprintln(os.Stderr, "hermind: starting in degraded mode. Chat will fail until you configure a provider.")
			// Replace primary with stub for TUI degraded mode only.
			deps.Provider = newStubProvider("unknown")
		} else {
			return err
		}
	}
```

Below that, continue the existing `ui.Run(...)` call but pass `deps.Provider`, `deps.AuxProvider`, `deps.Storage`, `deps.ToolReg`, `deps.SkillsReg`, `deps.AgentCfg` — not the old local variables.

- [ ] **Step 4: Write a smoke test**

Create `cli/engine_deps_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func TestBuildEngineDeps_Smoke(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "h.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Set a throwaway HOME so any file tool inits don't blow up.
	t.Setenv("HOME", tmp)

	cfg := &config.Config{}
	cfg.Provider.Provider = "anthropic"
	cfg.Provider.APIKey = "sk-test" // avoids errMissingAPIKey

	deps, err := BuildEngineDeps(cfg, store)
	if err != nil {
		t.Fatalf("BuildEngineDeps: %v", err)
	}
	if deps.Provider == nil {
		t.Error("Provider nil")
	}
	if deps.Storage == nil {
		t.Error("Storage nil")
	}
	if deps.ToolReg == nil {
		t.Error("ToolReg nil")
	}
	_ = os.Stdout // silence unused
}
```

- [ ] **Step 5: Build + test**

```bash
go build ./...
go test ./cli -run TestBuildEngineDeps -v
```

Expected: exit 0, test PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/engine_deps.go cli/engine_deps_test.go cli/repl.go
git commit -m "refactor(cli): extract BuildEngineDeps shared by repl and web"
```

---

### Task 7: ServerOpts + StreamHub adapter + registry wiring

Phase 1 needs the `api.Server` to hold a `*SessionRegistry` and a pre-built `sessionrun.Deps`. Also bridge `api.StreamHub` ↔ `sessionrun.EventPublisher`.

**Files:**
- Modify: `api/server.go`

- [ ] **Step 1: Add a StreamHub → EventPublisher adapter**

Append to `api/server.go` (or a new `api/sessionrun_bridge.go`):

```go
// hubPublisher adapts a *MemoryStreamHub to sessionrun.EventPublisher.
type hubPublisher struct{ hub *MemoryStreamHub }

func (h *hubPublisher) Publish(e sessionrun.Event) {
	h.hub.Publish(StreamEvent{
		Type:      e.Type,
		SessionID: e.SessionID,
		Data:      e.Data,
	})
}
```

Add `"github.com/odysseythink/hermind/api/sessionrun"` to the imports.

- [ ] **Step 2: Extend ServerOpts**

In `api/server.go`, add to `ServerOpts`:

```go
	// Deps is the pre-built Engine dependency bundle. Callers
	// (cli/web.go) fill this via cli.BuildEngineDeps. Required for the
	// POST /sessions/{id}/messages endpoint; zero-value leaves the
	// endpoint returning 503.
	Deps sessionrun.Deps
```

And on the `Server` struct:

```go
	registry *SessionRegistry
	deps     sessionrun.Deps
```

- [ ] **Step 3: Wire in NewServer**

In `NewServer`, after the existing field copies:

```go
	s.registry = NewSessionRegistry()
	s.deps = opts.Deps
	// Adapt the hub into the publisher sessionrun.Run expects.
	if s.opts.Streams != nil {
		s.deps.Hub = &hubPublisher{hub: s.opts.Streams}
	}
```

- [ ] **Step 4: Build**

```bash
go build ./api
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add api/server.go
git commit -m "feat(api): ServerOpts.Deps + registry + hub publisher bridge"
```

---

### Task 8: POST /messages handler

**Files:**
- Create: `api/handlers_session_run.go`
- Create: `api/handlers_session_run_test.go`
- Modify: `api/dto.go`
- Modify: `api/server.go` (register route)

- [ ] **Step 1: Add DTOs**

Append to `api/dto.go`:

```go
// MessageSubmitRequest is the body of POST /api/sessions/{id}/messages.
type MessageSubmitRequest struct {
	Text  string `json:"text"`
	Model string `json:"model,omitempty"`
}

// MessageSubmitResponse is returned on 202.
type MessageSubmitResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}
```

- [ ] **Step 2: Write failing tests**

Create `api/handlers_session_run_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/api/sessionrun"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
)

func buildTestServerWithDeps(t *testing.T, cfg *config.Config, deps sessionrun.Deps) *Server {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	srv, err := NewServer(&ServerOpts{
		Config:  cfg,
		Storage: nil, // storage calls are tolerated as nil in Phase 1 tests
		Token:   "t",
		Streams: NewMemoryStreamHub(),
		Deps:    deps,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

type slowProvider struct{ block chan struct{} }

func (p *slowProvider) Name() string { return "slow" }
func (p *slowProvider) Complete(ctx context.Context, req *provider.CompleteRequest) (*message.Message, error) {
	<-p.block
	return &message.Message{Role: message.RoleAssistant, Content: message.NewTextContent("ok")}, nil
}
func (p *slowProvider) Stream(ctx context.Context, req *provider.CompleteRequest, cb func(*provider.StreamDelta)) (*message.Message, error) {
	return p.Complete(ctx, req)
}

func TestMessagesPost_Accepted(t *testing.T) {
	cfg := &config.Config{}
	cfg.Provider.APIKey = "sk-test"
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: make(chan struct{})},
		ToolReg:  tool.NewRegistry(),
	}
	srv := buildTestServerWithDeps(t, cfg, deps)

	body, _ := json.Marshal(MessageSubmitRequest{Text: "hi"})
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != 202 {
		t.Fatalf("code = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	var resp MessageSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.SessionID != "s1" || resp.Status != "accepted" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestMessagesPost_MissingText(t *testing.T) {
	cfg := &config.Config{}
	cfg.Provider.APIKey = "sk-test"
	srv := buildTestServerWithDeps(t, cfg, sessionrun.Deps{ToolReg: tool.NewRegistry()})

	req := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestMessagesPost_InvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	cfg.Provider.APIKey = "sk-test"
	srv := buildTestServerWithDeps(t, cfg, sessionrun.Deps{ToolReg: tool.NewRegistry()})
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader([]byte(`{`)))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestMessagesPost_NoProvider(t *testing.T) {
	cfg := &config.Config{} // no APIKey
	srv := buildTestServerWithDeps(t, cfg, sessionrun.Deps{})
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages",
		bytes.NewReader([]byte(`{"text":"hi"}`)))
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestMessagesPost_Busy(t *testing.T) {
	cfg := &config.Config{}
	cfg.Provider.APIKey = "sk-test"
	block := make(chan struct{})
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: block},
		ToolReg:  tool.NewRegistry(),
	}
	srv := buildTestServerWithDeps(t, cfg, deps)

	// First POST launches a goroutine that blocks in slowProvider.
	reqBody := []byte(`{"text":"hi"}`)
	r1 := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader(reqBody))
	r1.Header.Set("Authorization", "Bearer t")
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, r1)
	if w1.Code != 202 {
		t.Fatalf("first code = %d, want 202", w1.Code)
	}
	// Wait for registry.Register to happen
	time.Sleep(20 * time.Millisecond)

	r2 := httptest.NewRequest("POST", "/api/sessions/s1/messages", bytes.NewReader(reqBody))
	r2.Header.Set("Authorization", "Bearer t")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, r2)
	if w2.Code != 409 {
		t.Fatalf("second code = %d, want 409", w2.Code)
	}
	close(block)
}

func TestMessagesPost_Unauthorized(t *testing.T) {
	cfg := &config.Config{}
	cfg.Provider.APIKey = "sk-test"
	srv := buildTestServerWithDeps(t, cfg, sessionrun.Deps{ToolReg: tool.NewRegistry()})
	req := httptest.NewRequest("POST", "/api/sessions/s1/messages",
		bytes.NewReader([]byte(`{"text":"hi"}`)))
	// no Authorization
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}
```

- [ ] **Step 3: Run tests — expect fail**

```bash
go test ./api -run TestMessagesPost -v
```

Expected: FAIL — route does not exist, all tests 404.

- [ ] **Step 4: Create the handler**

Create `api/handlers_session_run.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/api/sessionrun"
)

func (s *Server) handleSessionMessagesPost(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeErr(w, http.StatusBadRequest, "missing session id")
		return
	}
	var req MessageSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Text == "" {
		writeErr(w, http.StatusBadRequest, "text is required")
		return
	}
	if s.opts.Config == nil || s.opts.Config.Provider.APIKey == "" {
		writeErr(w, http.StatusServiceUnavailable,
			"provider not configured; open Config panel to set api_key")
		return
	}
	if !s.registry.Register(sessionID, func() {}) {
		// Duplicate Register attempt → either it's busy, or we raced
		// with a prior Clear. Check IsBusy explicitly.
		if s.registry.IsBusy(sessionID) {
			writeErr(w, http.StatusConflict, "session busy")
			return
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Replace the no-op cancel with the real one. Safe because Register
	// succeeded above and no other goroutine has dispatched Cancel yet.
	s.registry.Clear(sessionID)
	if !s.registry.Register(sessionID, cancel) {
		// Extremely unlikely race: someone else registered in between.
		cancel()
		writeErr(w, http.StatusConflict, "session busy")
		return
	}

	go func() {
		defer s.registry.Clear(sessionID)
		_ = sessionrun.Run(ctx, s.deps, sessionrun.Request{
			SessionID:   sessionID,
			UserMessage: req.Text,
			Model:       req.Model,
		})
	}()

	writeJSON(w, MessageSubmitResponse{SessionID: sessionID, Status: "accepted"})
	// writeJSON writes 200 by default; explicitly set 202 before body.
	// (Reorder: write header first; see Step 5 note.)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

> **Header order fix:** the handler above calls `writeJSON` which internally writes a 200 header before the body. Replace that call with:
> ```go
> w.Header().Set("Content-Type", "application/json")
> w.WriteHeader(http.StatusAccepted)
> _ = json.NewEncoder(w).Encode(MessageSubmitResponse{SessionID: sessionID, Status: "accepted"})
> ```

Also simplify the double-Register dance to a single atomic attempt by adding a `RegisterWithCancel` method to `SessionRegistry`. OR keep as shown and accept the minor race-free-but-ugly pattern. Simplest fix: change `Register` signature.

**Preferred:** edit `api/session_registry.go` to let the caller pass a cancel that can be set later — but that's heavier than just doing:

```go
// Replace the double-Register block with:
ctx, cancel := context.WithCancel(context.Background())
if !s.registry.Register(sessionID, cancel) {
	cancel()
	writeErr(w, http.StatusConflict, "session busy")
	return
}
```

— which is correct as written (cancel is the real func from the start). Adjust the handler accordingly before building.

- [ ] **Step 5: Register the route**

In `api/server.go`'s `r.Route("/api", …)` block, **after** the existing `/sessions/{id}/stream/sse` line, add:

```go
		r.Post("/sessions/{id}/messages", s.handleSessionMessagesPost)
```

- [ ] **Step 6: Run tests**

```bash
go test ./api -run TestMessagesPost -v
```

Expected: all six subtests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/handlers_session_run.go api/handlers_session_run_test.go api/dto.go api/server.go
git commit -m "feat(api): POST /sessions/{id}/messages dispatches engine"
```

---

### Task 9: POST /cancel handler

**Files:**
- Modify: `api/handlers_session_run.go` (add handler)
- Modify: `api/handlers_session_run_test.go` (add tests)
- Modify: `api/server.go` (register route)

- [ ] **Step 1: Write failing tests**

Append to `api/handlers_session_run_test.go`:

```go
func TestCancelPost_Running(t *testing.T) {
	cfg := &config.Config{}
	cfg.Provider.APIKey = "sk-test"
	block := make(chan struct{})
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: block},
		ToolReg:  tool.NewRegistry(),
	}
	srv := buildTestServerWithDeps(t, cfg, deps)

	// Start a run
	r1 := httptest.NewRequest("POST", "/api/sessions/s1/messages",
		bytes.NewReader([]byte(`{"text":"hi"}`)))
	r1.Header.Set("Authorization", "Bearer t")
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, r1)
	time.Sleep(20 * time.Millisecond)

	// Cancel
	r2 := httptest.NewRequest("POST", "/api/sessions/s1/cancel", nil)
	r2.Header.Set("Authorization", "Bearer t")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, r2)
	if w2.Code != 204 {
		t.Fatalf("code = %d, want 204", w2.Code)
	}
	close(block)
}

func TestCancelPost_NotRunning(t *testing.T) {
	srv := buildTestServerWithDeps(t, nil, sessionrun.Deps{})
	req := httptest.NewRequest("POST", "/api/sessions/nobody/cancel", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("code = %d, want 404", w.Code)
	}
}

func TestCancelPost_Idempotent(t *testing.T) {
	cfg := &config.Config{}
	cfg.Provider.APIKey = "sk-test"
	block := make(chan struct{})
	deps := sessionrun.Deps{
		Provider: &slowProvider{block: block},
		ToolReg:  tool.NewRegistry(),
	}
	srv := buildTestServerWithDeps(t, cfg, deps)
	r1 := httptest.NewRequest("POST", "/api/sessions/s1/messages",
		bytes.NewReader([]byte(`{"text":"hi"}`)))
	r1.Header.Set("Authorization", "Bearer t")
	srv.Router().ServeHTTP(httptest.NewRecorder(), r1)
	time.Sleep(20 * time.Millisecond)

	// First cancel: 204
	c1 := httptest.NewRequest("POST", "/api/sessions/s1/cancel", nil)
	c1.Header.Set("Authorization", "Bearer t")
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, c1)
	if w1.Code != 204 {
		t.Fatalf("first cancel = %d, want 204", w1.Code)
	}
	// Give the goroutine a moment to finish
	close(block)
	time.Sleep(20 * time.Millisecond)
	c2 := httptest.NewRequest("POST", "/api/sessions/s1/cancel", nil)
	c2.Header.Set("Authorization", "Bearer t")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, c2)
	if w2.Code != 404 {
		t.Fatalf("second cancel = %d, want 404", w2.Code)
	}
}
```

- [ ] **Step 2: Run — expect fail**

```bash
go test ./api -run TestCancelPost -v
```

Expected: 404 (route doesn't exist yet).

- [ ] **Step 3: Implement handler**

Append to `api/handlers_session_run.go`:

```go
func (s *Server) handleSessionCancel(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeErr(w, http.StatusBadRequest, "missing session id")
		return
	}
	if !s.registry.Cancel(sessionID) {
		writeErr(w, http.StatusNotFound, "session not running")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Register in `api/server.go`, right after `s.handleSessionMessagesPost`:

```go
		r.Post("/sessions/{id}/cancel", s.handleSessionCancel)
```

- [ ] **Step 4: Run**

```bash
go test ./api -run TestCancelPost -v
```

Expected: all three subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_session_run.go api/handlers_session_run_test.go api/server.go
git commit -m "feat(api): POST /sessions/{id}/cancel stops the running engine"
```

---

### Task 10: cli/web.go wires BuildEngineDeps

**Files:**
- Modify: `cli/web.go`

- [ ] **Step 1: Wire deps into ServerOpts**

In `cli/web.go`, `RunE` body, after `ensureStorage(app)`:

```go
	deps, err := BuildEngineDeps(app.Config, app.Storage)
	if err != nil {
		// Web mode does not fall back to a stub provider; let
		// NewFeishuApp-style init errors bubble up. errMissingAPIKey
		// is allowed — handlers return 503 from the /messages path.
		if !errors.Is(err, errMissingAPIKey) {
			return err
		}
	}
```

Pass `Deps: deps` into the `api.NewServer(&api.ServerOpts{...})` call.

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Manual smoke (optional)**

Start the server locally:

```bash
go run . web --exit-after 5s
```

Expected: "hermind web listening on …" prints; exits cleanly after 5s.

- [ ] **Step 4: Commit**

```bash
git add cli/web.go
git commit -m "feat(cli/web): build engine deps + pass into ServerOpts"
```

---

### Task 11: Full package regression

- [ ] **Step 1: Run the whole suite**

```bash
go test ./... -count=1
```

Expected: all packages pass. If anything fails, trace to whether it's a pre-existing issue on main (check with `git stash && git checkout main && go test <pkg> && git checkout - && git stash pop`) or a regression from this work.

- [ ] **Step 2: Run race detector on the new packages**

```bash
go test -race ./api ./api/sessionrun ./cli -count=1
```

Expected: no race warnings.

---

### Task 12: CHANGELOG

- [ ] **Step 1: Add entry**

Open `CHANGELOG.md`. Under `## Unreleased`, above any existing `### Breaking`, add a new section:

```markdown
### Added

- `POST /api/sessions/{id}/messages` — submit a user message, Engine
  runs in a goroutine, events stream via the existing WebSocket / SSE
  endpoints. Returns 202 on acceptance, 409 if a run is already in
  flight for that session, 503 if no provider is configured.
- `POST /api/sessions/{id}/cancel` — stop the running Engine for a
  session. Returns 204 on cancel, 404 if the session is not running
  (idempotent).
- `api/sessionrun` package — shared runner between TUI (legacy) and
  web. `cli.BuildEngineDeps` builds the shared provider/tool/skills
  bundle.
```

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): phase 1 web chat backend endpoints"
```

---

## Self-review checklist

- [ ] `grep -rn 'sessionrun' api/ cli/` returns matches only in the files this plan touched.
- [ ] `go test ./... -count=1` passes.
- [ ] `go test -race ./api ./api/sessionrun ./cli` passes.
- [ ] `go vet ./...` clean.
- [ ] The TUI still runs: `go run . run` opens the TUI as before (no behavior change — we only moved code, didn't remove).
- [ ] POST /messages with missing Auth → 401. With invalid JSON → 400. With no provider → 503.
- [ ] POST /cancel on idle session → 404.
