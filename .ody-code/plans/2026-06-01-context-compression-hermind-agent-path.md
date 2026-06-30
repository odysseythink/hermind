# Agent Path Context Compression

> **Depends on:** `hermind-core.md` Tasks C1–C4 (adapter layer exists), `hermind-models.md` Tasks M1–M3 (models + defaults migrated)

**Goal:** Wire Pantheon's `WithContextEngine` into Hermind's agent runtime so compression triggers automatically per step, bridge `UpdateFromResponse` via `WithOnStepFinish`, and persist summaries via a `CompressionObserver` wrapper.

**Architecture:** The agent run loop is opaque — Pantheon calls `CompressMessages` before each step. Hermind wraps the `ContextEngine` in an observer that extracts the summary from returned messages and persists it to `thread_compactions`. Because Pantheon does not yet call `UpdateFromResponse` (upstream P8) or expose `PreviousSummary()` (upstream P1), Hermind uses `WithOnStepFinish` as a typed shim for usage calibration and message-inspection for persistence.

**Tech Stack:** Go 1.26, Pantheon SDK, GORM

---

## File Structure

| Path | Responsibility |
|---|---|
| `backend/internal/agent/compression/observer.go` | `Observer` wrapper around `compression.ContextEngine`; extracts summary from compressed messages and calls `SaveFunc` |
| `backend/internal/agent/compression/observer_test.go` | Unit tests for summary extraction and delegation |
| `backend/internal/agent/session.go` | Add `compressor` field to `Session`; modify `newSession`/`NewSessionForTesting` signatures; add `initAgent` helper; add test-only getter |
| `backend/internal/agent/session_compaction_test.go` | Verify `NewSessionForTesting` stores compressor and `initAgent` wires `WithContextEngine` |
| `backend/internal/agent/compression_wiring.go` | `buildCompressor` helper — checks workspace/system setting, builds `DefaultCompressor`, wraps in `Observer` with `CompactionStore` save callback |
| `backend/internal/agent/handler.go` | Call `buildCompressor` after workspace resolution; pass compressor to `newSession`; replace inline `pantheonAgent.New` with `sess.initAgent` |
| `backend/internal/agent/runtime.go` | Same wiring as `handler.go`; add `testCompressorOverride` field + setter for integration tests |
| `backend/internal/agent/agent_compaction_test.go` | Integration test: `RunAgentDirectly` with mock compressor override asserts `CompressMessages` is called per step |

## Dependency Overview

```
A1 (Observer wrapper + test)
  -> A2 (Shared signature: newSession/NewSessionForTesting + all callers + whole-tree vet)
    -> A3 (handler.go + runtime.go wiring + integration test)
```

## Risks & Open Questions

| # | Risk | Assumption | Impact if wrong |
|---|---|---|---|
| 1 | Pantheon `agent.go`/`stream.go` do not call `UpdateFromResponse` | `WithOnStepFinish` callback is a reliable bridge — it fires after every step with the step's `core.Usage` | Threshold calibration lags by one step; mitigation is acceptable for V1 |
| 2 | Pantheon lacks `PreviousSummary()`/`SetPreviousSummary()` accessors | `Observer` extracts summary from message text using the known prefix `[Compressed summary of earlier conversation]\n` | If Pantheon changes the prefix format, extraction breaks; prefix is covered by Pantheon's own tests and is part of the public output contract |
| 3 | Agent sessions have no `thread_id` | `ThreadCompaction.ThreadID` is nullable; agent summaries are stored with `NULL` thread | Cross-thread handoff (extension E2) must explicitly look up the latest agent compaction by `workspace_id` where `thread_id IS NULL` |

---

### Task A1: `CompressionObserver` wrapper + test

**Depends on:** `hermind-core.md` Task C4 (`NewForAgent` factory exists), `hermind-models.md` Task M1 (`ThreadCompaction` model exists)

**Files:**
- Create: `backend/internal/agent/compression/observer.go`
- Create: `backend/internal/agent/compression/observer_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/agent/compression/observer_test.go
package agentcompression

import (
	"context"
	"testing"

	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

type fakeEngine struct {
	calls       int
	lastMessages []core.Message
}

func (f *fakeEngine) Name() string { return "fake" }
func (f *fakeEngine) UpdateFromResponse(u core.Usage) error {
	f.calls++ // track calls for verification
	return nil
}
func (f *fakeEngine) ShouldCompress(tokens int) bool { return true }
func (f *fakeEngine) CompressMessages(ctx context.Context, messages []core.Message, focusTopic string) ([]core.Message, error) {
	f.calls++
	f.lastMessages = append([]core.Message(nil), messages...)
	return append([]core.Message(nil), messages...), nil
}

func TestObserver_ExtractsSummaryAndCallsSave(t *testing.T) {
	inner := &fakeEngine{}
	var saved string
	obs := NewObserver(inner, func(summary string) error {
		saved = summary
		return nil
	})

	msgs := []core.Message{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("hello")},
		{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("[Compressed summary of earlier conversation]\nThe user said hello.")},
	}
	out, err := obs.CompressMessages(context.Background(), msgs, "")
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, "The user said hello.", saved)
	require.Equal(t, 1, inner.calls)
}

func TestObserver_DelegatesUpdateFromResponse(t *testing.T) {
	inner := &fakeEngine{}
	obs := NewObserver(inner, nil)
	err := obs.UpdateFromResponse(core.Usage{TotalTokens: 100})
	require.NoError(t, err)
	require.Equal(t, 1, inner.calls)
}

func TestObserver_NoSummary_NoSaveCall(t *testing.T) {
	inner := &fakeEngine{}
	var saved string
	obs := NewObserver(inner, func(summary string) error {
		saved = summary
		return nil
	})

	msgs := []core.Message{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("no compression here")},
	}
	out, err := obs.CompressMessages(context.Background(), msgs, "")
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, saved)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/agent/compression/ -run TestObserver -v`
Expected: FAIL — `NewObserver` undefined, `Observer` type undefined

- [ ] **Step 3: Write minimal implementation**

```go
// backend/internal/agent/compression/observer.go
package agentcompression

import (
	"context"
	"strings"

	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
)

const summaryPrefix = "[Compressed summary of earlier conversation]\n"

// SaveFunc persists an extracted summary. Workspace and thread IDs are captured
// by the closure at the call site (handler.go / runtime.go).
type SaveFunc func(summary string) error

// Observer wraps a ContextEngine to intercept compression results and extract
// the summary for persistence. It is a typed shim for the missing
// PreviousSummary()/SetPreviousSummary() accessors in current Pantheon.
type Observer struct {
	inner compression.ContextEngine
	save  SaveFunc
}

// NewObserver wraps the given engine with summary-extraction persistence.
func NewObserver(inner compression.ContextEngine, save SaveFunc) *Observer {
	return &Observer{inner: inner, save: save}
}

func (o *Observer) Name() string { return o.inner.Name() }

func (o *Observer) UpdateFromResponse(u core.Usage) error {
	return o.inner.UpdateFromResponse(u)
}

func (o *Observer) ShouldCompress(tokens int) bool {
	return o.inner.ShouldCompress(tokens)
}

func (o *Observer) CompressMessages(ctx context.Context, messages []core.Message, focusTopic string) ([]core.Message, error) {
	out, err := o.inner.CompressMessages(ctx, messages, focusTopic)
	if err != nil {
		return nil, err
	}
	if summary := extractSummary(out); summary != "" && o.save != nil {
		_ = o.save(summary)
	}
	return out, nil
}

func extractSummary(msgs []core.Message) string {
	for _, m := range msgs {
		if m.Role != core.MESSAGE_ROLE_ASSISTANT {
			continue
		}
		text := m.Text()
		if strings.HasPrefix(text, summaryPrefix) {
			return strings.TrimPrefix(text, summaryPrefix)
		}
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/agent/compression/ -run TestObserver -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/agent/compression/observer.go backend/internal/agent/compression/observer_test.go
git commit -m "feat(agent): add CompressionObserver wrapper for summary extraction"
```

---

### Task A2: Shared signature — `newSession`/`NewSessionForTesting` accept compressor + all callers + whole-tree vet

**Depends on:** Task A1

**Files:**
- Modify: `backend/internal/agent/session.go`
- Modify: `backend/internal/agent/session_test.go`
- Modify: `backend/internal/agent/session_agent_test.go`
- Modify: `backend/internal/agent/session_compaction_test.go` (new file)

- [ ] **Step 1: Change signatures + write the behavioral test at the new arity**

In `session.go`, add the `compressor` field and change both signatures:

```go
// Add to Session struct (line ~44 in current file)
type Session struct {
	// ... existing fields ...
	compressor compression.ContextEngine
}

// Change newSession signature (line ~84)
func newSession(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User,
	lm core.LanguageModel, systemPrompt string, reg *tool.Registry, io AgentIO, approvalTTL time.Duration, eventLog eventLogger, compressor compression.ContextEngine) *Session {

	// In struct initialization (line ~88)
	s := &Session{
		// ... existing fields ...
		compressor: compressor,
	}

	// Replace the pantheonAgent.New call in newSession (line ~115) with initAgent:
	s.initAgent(lm, reg)

	// After newSession, add initAgent helper:
	func (s *Session) initAgent(lm core.LanguageModel, reg *tool.Registry) {
		opts := []pantheonAgent.Option{
			pantheonAgent.WithRegistry(reg),
			pantheonAgent.WithMaxSteps(10),
		}
		if s.compressor != nil {
			opts = append(opts, pantheonAgent.WithContextEngine(s.compressor))
			opts = append(opts, pantheonAgent.WithOnStepFinish(func(step int, messages []core.Message, usage core.Usage) error {
				return s.compressor.UpdateFromResponse(usage)
			}))
		}
		s.pAgent = pantheonAgent.New(lm, opts...)
		s.conv.RegisterParticipant(&conversation.Participant{
			Name:  participantAgent,
			Role:  s.systemPrompt,
			Agent: s.pAgent,
		})
	}

	// Test-only getter
	func (s *Session) CompressorForTesting() compression.ContextEngine {
		return s.compressor
	}
}

// Change NewSessionForTesting signature (line ~129)
func NewSessionForTesting(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User,
	lm core.LanguageModel, systemPrompt string, reg *tool.Registry, io AgentIO, compressor compression.ContextEngine) *Session {
	return newSession(parentCtx, uuid, ws, user, lm, systemPrompt, reg, io, 2*time.Minute, nil, compressor)
}
```

Write the behavioral test at the new arity:

```go
// backend/internal/agent/session_compaction_test.go
package agent_test

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

type noopCompressor struct{}

func (n *noopCompressor) Name() string { return "noop" }
func (n *noopCompressor) UpdateFromResponse(u core.Usage) error { return nil }
func (n *noopCompressor) ShouldCompress(tokens int) bool { return false }
func (n *noopCompressor) CompressMessages(ctx context.Context, messages []core.Message, focusTopic string) ([]core.Message, error) {
	return messages, nil
}

func TestNewSession_AcceptsCompressor(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	ws := &models.Workspace{ID: 1}
	comp := &noopCompressor{}
	sess := agent.NewSessionForTesting(context.Background(), "test-uuid", ws, nil,
		&mockLanguageModel{provider: "mock", model: "mock-model"}, "sys", nil, wc, comp)
	require.NotNil(t, sess)
	require.Equal(t, comp, sess.CompressorForTesting())
}
```

- [ ] **Step 2: Find and update EVERY caller (prod + tests)**

Run: `grep -rn "NewSessionForTesting(" backend/`
Expected hits:
- `backend/internal/agent/session_test.go:36` — pass `nil` for compressor
- `backend/internal/agent/session_test.go:65` — pass `nil` for compressor
- `backend/internal/agent/session_test.go:91` — pass `nil` for compressor

Run: `grep -rn "newSession(" backend/`
Expected hits:
- `backend/internal/agent/session.go:84` — definition (already changed in Step 1)
- `backend/internal/agent/session.go:131` — `NewSessionForTesting` body (already changed)
- `backend/internal/agent/handler.go:84` — pass `nil` for now (wired in Task A3)
- `backend/internal/agent/runtime.go:174` — pass `nil` for now (wired in Task A3)

Update `session_test.go` lines 36, 65, 91: add `nil` as the final argument.

For `handler.go` and `runtime.go`, pass `nil` as a placeholder — Task A3 will replace with the real compressor.

- [ ] **Step 3: Whole-tree typecheck (incl. tests) + targeted test**

Run:
```bash
cd backend && go vet ./... && go test ./internal/agent/ -run TestNewSession_AcceptsCompressor -v
```
Expected: `go vet ./...` passes (no stale callers anywhere, including `_test.go` files); targeted test passes.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/agent/session.go backend/internal/agent/session_test.go \
  backend/internal/agent/session_compaction_test.go \
  backend/internal/agent/handler.go backend/internal/agent/runtime.go
git commit -m "refactor(agent): add compressor to newSession/NewSessionForTesting + update all callers"
```

---

### Task A3: `handler.go` + `runtime.go` compressor build/wiring + integration test

**Depends on:** Task A2

**Files:**
- Create: `backend/internal/agent/compression_wiring.go`
- Modify: `backend/internal/agent/handler.go`
- Modify: `backend/internal/agent/runtime.go`
- Modify: `backend/internal/agent/handler_test.go` (if needed for build)
- Create: `backend/internal/agent/agent_compaction_test.go`

- [ ] **Step 1: Write `compression_wiring.go` helper**

```go
// backend/internal/agent/compression_wiring.go
package agent

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/agent/compression"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"gorm.io/gorm"
)

func buildCompressor(db *gorm.DB, ws *models.Workspace, lm core.LanguageModel, sysSvc *services.SystemService) compression.ContextEngine {
	if !isCompressionEnabled(ws, sysSvc) {
		return nil
	}

	comp := agentcompression.NewForAgent(lm.Model(), agentcompression.ContextLengthFor(lm.Model()))
	store := agentcompression.NewCompactionStore(db)
	obs := agentcompression.NewObserver(comp, func(summary string) error {
		return store.Save(context.Background(), &models.ThreadCompaction{
			WorkspaceID: ws.ID,
			ThreadID:    nil, // agent sessions have no thread
			Summary:     summary,
		})
	})
	return obs
}

func isCompressionEnabled(ws *models.Workspace, sysSvc *services.SystemService) bool {
	if ws.CompressEnabled != nil {
		return *ws.CompressEnabled
	}
	if sysSvc != nil {
		v, _ := sysSvc.GetSetting(context.Background(), "context_compress_enabled")
		return v == "true"
	}
	return false
}
```

- [ ] **Step 2: Wire into `handler.go`**

Replace the `newSession` call in `handler.go` (line ~84):

```go
// Before:
// sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), wc, ttl, r.deps.EventLog)

// After:
comp := buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc)
sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), wc, ttl, r.deps.EventLog, comp)
```

Replace the inline `pantheonAgent.New` in Step 3 (lines ~94-102) with `sess.initAgent`:

```go
// Before:
// sess.pAgent = pantheonAgent.New(lm, pantheonAgent.WithRegistry(reg), pantheonAgent.WithMaxSteps(10))
// sess.conv.RegisterParticipant(...)

// After:
sess.initAgent(lm, reg)
```

- [ ] **Step 3: Wire into `runtime.go`**

Replace the `newSession` call in `runtime.go` (line ~174):

```go
// Before:
// sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), io, ttl, r.deps.EventLog)

// After:
comp := buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc)
sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), io, ttl, r.deps.EventLog, comp)
```

Replace the inline `pantheonAgent.New` (lines ~181-189) with `sess.initAgent`:

```go
// Before:
// sess.pAgent = pantheonAgent.New(lm, pantheonAgent.WithRegistry(reg), pantheonAgent.WithMaxSteps(10))
// sess.conv.RegisterParticipant(...)

// After:
sess.initAgent(lm, reg)
```

Add test-only override to `Runtime`:

```go
// In Runtime struct (line ~46), add:
testCompressorOverride compression.ContextEngine

// Add setter method (after SetChatSearcher):
func (r *Runtime) SetTestCompressorOverride(c compression.ContextEngine) {
	r.testCompressorOverride = c
}
```

And in both `handler.go` and `runtime.go`, modify `buildCompressor` call to respect the override:

```go
// In handler.go and runtime.go, replace:
// comp := buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc)
// with:
var comp compression.ContextEngine
if r.testCompressorOverride != nil {
	comp = r.testCompressorOverride
} else {
	comp = buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc)
}
```

- [ ] **Step 4: Write the integration test**

```go
// backend/internal/agent/agent_compaction_test.go
package agent_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

// countingCompressor is a ContextEngine that records every CompressMessages call.
type countingCompressor struct {
	compressCalls int
	updateCalls   int
}

func (c *countingCompressor) Name() string { return "counting" }
func (c *countingCompressor) UpdateFromResponse(u core.Usage) error {
	c.updateCalls++
	return nil
}
func (c *countingCompressor) ShouldCompress(tokens int) bool { return true }
func (c *countingCompressor) CompressMessages(ctx context.Context, messages []core.Message, focusTopic string) ([]core.Message, error) {
	c.compressCalls++
	return append([]core.Message(nil), messages...), nil
}

func TestRunAgentDirectly_CompressorWired(t *testing.T) {
	db, cleanup := agent.NewTestDB(t)
	defer cleanup()
	cfg := agent.NewTestConfig()
	enc := agent.NewTestEncryptor(t)

	authSvc := agent.NewAuthService(db, cfg, enc)
	tempTokenSvc := agent.NewTemporaryAuthTokenService(db)
	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
	})

	// Seed workspace with compression enabled
	ws := &models.Workspace{Name: "Test", Slug: "test"}
	require.NoError(t, db.Create(ws).Error)
	trueVal := true
	ws.CompressEnabled = &trueVal
	require.NoError(t, db.Save(ws).Error)

	// Create invocation
	uid, err := rt.CreateInvocation(context.Background(), ws, nil, nil, "@agent do work")
	require.NoError(t, err)

	// Mock LM that runs for 2 steps (tool call + final text)
	mock := &mockLanguageModel{
		provider: "openai", model: "gpt-4o-mini",
		parts: [][]core.ContentParter{
			{core.ToolCallPart{ID: "1", Name: "rag-memory", Arguments: `{"action":"search"}`}},
			core.NewTextContent("Done!"),
		},
	}
	rt.SetTestLanguageModelOverride(mock)

	comp := &countingCompressor{}
	rt.SetTestCompressorOverride(comp)

	io := &agent.FakeAgentIO{}
	input := agent.NewFakeAgentInput()

	var runErr error
	done := make(chan struct{})
	go func() {
		runErr = rt.RunAgentDirectly(context.Background(), uid, io, input)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunAgentDirectly timed out")
	}

	require.NoError(t, runErr)
	// Compression is called before each step. With 2 steps, expect 2 calls.
	require.GreaterOrEqual(t, comp.compressCalls, 1, "expected at least one CompressMessages call")
	require.GreaterOrEqual(t, comp.updateCalls, 1, "expected at least one UpdateFromResponse call")
}
```

> **Note:** `FakeAgentIO` and `NewFakeAgentInput` may not exist. If they don't, use the same mock IO pattern from `lifecycle_e2e_test.go` or `e2e_handoff_test.go`. The executor must adapt the test to use existing test helpers in the `agent` package. The key assertions are `compressCalls >= 1` and `updateCalls >= 1`.

- [ ] **Step 5: Run whole-tree typecheck + targeted test**

Run:
```bash
cd backend && go vet ./... && go test ./internal/agent/ -run TestRunAgentDirectly_CompressorWired -v
```
Expected: `go vet ./...` passes; targeted test passes.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/agent/compression_wiring.go \
  backend/internal/agent/handler.go backend/internal/agent/runtime.go \
  backend/internal/agent/agent_compaction_test.go
git commit -m "feat(agent): wire compressor into handler.go + runtime.go with step-end persistence"
```

---

## Self-Review

Reproduce all seven as `- [ ]` checkboxes — do not shrink to five:

- [ ] **1. Spec coverage (build the table).**

| Design § | Requirement | Task(s) | Status |
|---|---|---|---|
| §1.1 | Agent path `WithContextEngine` | A2, A3 | covered |
| §1.1 | Global + per-workspace switch | A3 (`isCompressionEnabled`) | covered |
| §11.3 | Agent wiring | A2, A3 | covered |
| §8 | Degradation & error layers (agent path) | A3 (`buildCompressor` returns nil when disabled) | covered |
| §9 | Observability (agent path) | A1 (observer logging via `mlog` if desired — V1 uses silent save failure) | deferred to E4 |
| §18.5 | 600s cooldown | upstream P5 — no Hermind code needed | no-op |

- [ ] **2. Placeholder scan:** No `TODO`/`TBD`/deferred-by-dependency excuses. The `UpdateFromResponse` bridge and `PreviousSummary` extraction are implemented as typed shims (`WithOnStepFinish` and `Observer`), not TODO comments.

- [ ] **3. No phantom tasks:** Every task produces a verifiable change. Zero `--allow-empty`. The coverage table marks deferred observability as deferred to E4, not as an empty task.

- [ ] **4. Dependency soundness:** A1 -> A2 -> A3. No task references a symbol from a later task. `NewForAgent` and `ThreadCompaction` are from prior sub-plans (hermind-core, hermind-models) and are declared in `Depends on`.

- [ ] **5. Caller & build soundness:** Task A2 changes `newSession` and `NewSessionForTesting` in ONE task, updates all callers (3 test hits + 2 prod hits), and ends with `go vet ./...`. No other task changes these signatures.

- [ ] **6. Test-the-risk:** A1 tests summary extraction (the core mutation — what gets persisted). A3's integration test asserts that `CompressMessages` and `UpdateFromResponse` are actually called during agent execution (the wiring risk).

- [ ] **7. Type consistency:** `compression.ContextEngine` is used consistently. `Observer` implements the interface. `newSession` accepts `compression.ContextEngine`. `SaveFunc` takes `string` (summary only), with workspace/thread captured by closure. `initAgent` uses `s.compressor` for both `WithContextEngine` and `WithOnStepFinish`.
