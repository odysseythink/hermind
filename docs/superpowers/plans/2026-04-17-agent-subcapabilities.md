# Agent Sub-Capabilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring three Python-side agent facilities into parity: (1) **iterative compressor** that recompresses progressively when one pass isn't enough; (2) **MemoryManager** that composes multiple `memprovider.Provider` backends plus a built-in "turn history" provider into one system-prompt contribution; (3) **AuxiliaryClient** that picks a healthy sibling provider for compression / vision / re-ranking tasks with a fallback chain (OpenRouter → Nous → Codex → Anthropic). These three show up together because the compressor depends on the auxiliary client, and the memory manager's output feeds into the compressor's protected head.

**Architecture:** Extend `agent/compression.go` with a `CompressIteratively` loop that keeps summarizing until the token estimate drops below `TargetRatio × ContextLength` or `MaxPasses` is hit. Add `agent/memory_manager.go` holding a `MemoryManager` struct that owns a `BuiltinProvider` (turn-history digest) plus zero-or-more external providers loaded via `memprovider.Factory`. Add `provider/auxiliary.go` implementing an `AuxClient` that wraps a `FallbackChain` with compressor-friendly defaults (tight budget, no tools). Each piece lands in its own file + test file so they can be reviewed and merged independently.

**Tech Stack:** Go 1.21+, existing `agent`, `provider`, `provider/fallback`, `tool/memory/memprovider`, `config` packages.

---

## File Structure

- Modify: `agent/compression.go` — add `CompressIteratively(ctx, history, budget)`; keep existing `Compress` for backward compat
- Modify: `agent/compression_test.go`
- Create: `agent/memory_manager.go` — `MemoryManager` + `BuiltinProvider`
- Create: `agent/memory_manager_test.go`
- Create: `provider/auxiliary.go` — `AuxClient` over a `FallbackChain`
- Create: `provider/auxiliary_test.go`
- Modify: `agent/engine.go` — construct `MemoryManager` + `AuxClient` when the aux provider is set; call `CompressIteratively` instead of `Compress` when history size warrants

---

## Task 1: CompressIteratively

**Files:**
- Modify: `agent/compression.go`
- Modify: `agent/compression_test.go`

- [ ] **Step 1: Write the failing test**

Append to `agent/compression_test.go`:

```go
func TestCompressIteratively_HonorsMaxPasses(t *testing.T) {
	aux := &stubCompressionProvider{summary: "short"}
	c := NewCompressor(config.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		TargetRatio: 0.1,
		ProtectLast: 2,
		MaxPasses:   2,
	}, aux)

	// Build a 50-message history.
	hist := make([]message.Message, 0, 50)
	for i := 0; i < 50; i++ {
		hist = append(hist, message.Message{
			Role:    message.RoleUser,
			Content: message.TextContent("lorem ipsum dolor sit amet consectetur adipiscing elit " + strings.Repeat("x", 100)),
		})
	}
	out, passes, err := c.CompressIteratively(context.Background(), hist, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if passes < 1 || passes > 2 {
		t.Errorf("passes = %d, want 1..2", passes)
	}
	if len(out) >= len(hist) {
		t.Errorf("expected shorter history, got %d -> %d", len(hist), len(out))
	}
}

func TestCompressIteratively_ShortHistoryReturnedUnchanged(t *testing.T) {
	aux := &stubCompressionProvider{summary: "s"}
	c := NewCompressor(config.CompressionConfig{Enabled: true, ProtectLast: 2, MaxPasses: 3}, aux)
	hist := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("hi")},
		{Role: message.RoleAssistant, Content: message.TextContent("hello")},
	}
	out, passes, err := c.CompressIteratively(context.Background(), hist, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if passes != 0 {
		t.Errorf("passes = %d", passes)
	}
	if len(out) != len(hist) {
		t.Errorf("len changed: %d -> %d", len(hist), len(out))
	}
}
```

Import `"strings"` if not already. `stubCompressionProvider` likely already exists in the file; if not, copy the pattern from `agent/engine_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./agent/ -run TestCompressIteratively -v`
Expected: FAIL — method undefined.

- [ ] **Step 3: Implement CompressIteratively**

Append to `agent/compression.go`:

```go
// CompressIteratively calls Compress repeatedly until the estimated
// token count of the result drops below budget, the pass count hits
// cfg.MaxPasses, or a pass produces no further reduction.
//
// Returns the final history, the number of passes executed, and any
// error from the underlying summarization call. Passes=0 means no
// compression was needed or possible.
func (c *Compressor) CompressIteratively(ctx context.Context, history []message.Message, budget int) ([]message.Message, int, error) {
	if !c.cfg.Enabled || c.aux == nil || budget <= 0 {
		return history, 0, nil
	}
	maxPasses := c.cfg.MaxPasses
	if maxPasses <= 0 {
		maxPasses = 3
	}

	current := history
	passes := 0
	lastLen := len(history)

	for i := 0; i < maxPasses; i++ {
		if estimateTokens(current) <= budget {
			return current, passes, nil
		}
		shorter, err := c.Compress(ctx, current)
		if err != nil {
			return current, passes, err
		}
		if len(shorter) >= lastLen {
			// No forward progress; bail out to avoid infinite loops.
			return shorter, passes, nil
		}
		current = shorter
		lastLen = len(current)
		passes++
	}
	return current, passes, nil
}

// estimateTokens returns a rough char-based token count across every
// message in history. ~4 chars/token keeps the estimate independent of
// the specific provider tokenizer.
func estimateTokens(history []message.Message) int {
	total := 0
	for _, m := range history {
		total += len(m.Content.Text()) / 4
	}
	return total
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./agent/ -run TestCompressIteratively -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add agent/compression.go agent/compression_test.go
git commit -m "feat(agent): iterative compression with pass budget"
```

---

## Task 2: MemoryManager + BuiltinProvider

**Files:**
- Create: `agent/memory_manager.go`
- Create: `agent/memory_manager_test.go`

- [ ] **Step 1: Write the failing test**

Create `agent/memory_manager_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

type fakeMemProvider struct {
	name     string
	recalled string
}

func (f *fakeMemProvider) Name() string { return f.name }
func (f *fakeMemProvider) SyncTurn(context.Context, memprovider.Turn) error { return nil }
func (f *fakeMemProvider) Recall(_ context.Context, _ string, _ int) (string, error) {
	return f.recalled, nil
}
func (f *fakeMemProvider) Close() error { return nil }

func TestMemoryManager_SystemPromptCombinesProviders(t *testing.T) {
	mm := NewMemoryManager(nil)
	mm.AddProvider(&fakeMemProvider{name: "honcho", recalled: "user likes markdown"})
	mm.AddProvider(&fakeMemProvider{name: "mem0", recalled: "prefers vim"})

	prompt, err := mm.BuildSystemPrompt(context.Background(), "write a report")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"user likes markdown", "prefers vim", "honcho", "mem0"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestMemoryManager_BuiltinDigestsRecentTurns(t *testing.T) {
	mm := NewMemoryManager(nil)
	mm.ObserveTurn(message.Message{Role: message.RoleUser, Content: message.TextContent("my name is Alice")})
	mm.ObserveTurn(message.Message{Role: message.RoleAssistant, Content: message.TextContent("nice to meet you Alice")})

	digest := mm.BuiltinDigest()
	if !strings.Contains(digest, "Alice") {
		t.Errorf("digest missing name: %s", digest)
	}
}

func TestMemoryManager_NoProviders_EmptyPrompt(t *testing.T) {
	mm := NewMemoryManager(nil)
	prompt, err := mm.BuildSystemPrompt(context.Background(), "anything")
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "" {
		t.Errorf("expected empty prompt, got %q", prompt)
	}
}

func TestMemoryManager_SyncTurnFansOut(t *testing.T) {
	p1 := &recordingMemProvider{name: "a"}
	p2 := &recordingMemProvider{name: "b"}
	mm := NewMemoryManager(nil)
	mm.AddProvider(p1)
	mm.AddProvider(p2)
	_ = mm.SyncTurn(context.Background(), memprovider.Turn{User: "hi", Assistant: "hello"})
	if p1.turns != 1 || p2.turns != 1 {
		t.Errorf("sync turn fan-out: p1=%d p2=%d", p1.turns, p2.turns)
	}
}

type recordingMemProvider struct {
	name  string
	turns int
}

func (r *recordingMemProvider) Name() string                              { return r.name }
func (r *recordingMemProvider) SyncTurn(context.Context, memprovider.Turn) error { r.turns++; return nil }
func (r *recordingMemProvider) Recall(context.Context, string, int) (string, error) { return "", nil }
func (r *recordingMemProvider) Close() error                              { return nil }
```

If `memprovider.Turn` is not the actual field name, adjust. Run `grep -n "type Turn\|type Provider" tool/memory/memprovider/*.go` to confirm.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./agent/ -run TestMemoryManager -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement MemoryManager**

Create `agent/memory_manager.go`:

```go
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// MemoryManager composes multiple memprovider.Provider backends plus
// a built-in turn-history digest into a single system-prompt
// contribution. Safe for concurrent use.
type MemoryManager struct {
	mu        sync.Mutex
	providers []memprovider.Provider
	recent    []message.Message // bounded ring of recent turns
	limit     int
}

// NewMemoryManager constructs a manager with optional seed providers.
func NewMemoryManager(initial []memprovider.Provider) *MemoryManager {
	return &MemoryManager{
		providers: append([]memprovider.Provider(nil), initial...),
		limit:     20,
	}
}

// AddProvider registers a new provider. Providers are queried in
// insertion order.
func (m *MemoryManager) AddProvider(p memprovider.Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, p)
}

// ObserveTurn records a message turn for the built-in digest.
func (m *MemoryManager) ObserveTurn(msg message.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recent = append(m.recent, msg)
	if len(m.recent) > m.limit {
		m.recent = m.recent[len(m.recent)-m.limit:]
	}
}

// BuiltinDigest returns a short, human-readable summary of the recent
// turns. Used as the always-on contribution even when no remote
// providers are configured.
func (m *MemoryManager) BuiltinDigest() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.recent) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Recent turns:\n")
	for _, msg := range m.recent {
		role := "user"
		if msg.Role == message.RoleAssistant {
			role = "assistant"
		}
		text := msg.Content.Text()
		if len(text) > 200 {
			text = text[:200] + "…"
		}
		fmt.Fprintf(&sb, "- %s: %s\n", role, text)
	}
	return sb.String()
}

// BuildSystemPrompt queries every provider with the given query and
// returns a concatenated contribution suitable for inclusion in the
// agent's system prompt. Provider failures are logged (via fmt to
// stderr in this minimal impl) and skipped — one failing backend must
// never break the prompt.
func (m *MemoryManager) BuildSystemPrompt(ctx context.Context, query string) (string, error) {
	m.mu.Lock()
	providers := append([]memprovider.Provider(nil), m.providers...)
	m.mu.Unlock()

	var sb strings.Builder
	for _, p := range providers {
		recalled, err := p.Recall(ctx, query, 5)
		if err != nil {
			fmt.Printf("memory: %s: recall failed: %v\n", p.Name(), err)
			continue
		}
		if recalled == "" {
			continue
		}
		fmt.Fprintf(&sb, "From %s memory:\n%s\n\n", p.Name(), recalled)
	}
	if digest := m.BuiltinDigest(); digest != "" {
		fmt.Fprintf(&sb, "%s\n", digest)
	}
	return strings.TrimSpace(sb.String()), nil
}

// SyncTurn fans a turn out to every provider. Errors are collected
// and returned as a single composite error (or nil).
func (m *MemoryManager) SyncTurn(ctx context.Context, t memprovider.Turn) error {
	m.mu.Lock()
	providers := append([]memprovider.Provider(nil), m.providers...)
	m.mu.Unlock()

	var errs []string
	for _, p := range providers {
		if err := p.SyncTurn(ctx, t); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("memory: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Close closes every provider. Errors are joined.
func (m *MemoryManager) Close() error {
	m.mu.Lock()
	providers := m.providers
	m.providers = nil
	m.mu.Unlock()
	var errs []string
	for _, p := range providers {
		if err := p.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("memory close: %s", strings.Join(errs, "; "))
	}
	return nil
}
```

If `memprovider.Provider` uses a different method set (e.g. `Name()`, `SyncTurn`, `Recall`, `Close`), adjust. Run `grep -n "type Provider interface" tool/memory/memprovider/*.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./agent/ -run TestMemoryManager -v -race`
Expected: PASS (4 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add agent/memory_manager.go agent/memory_manager_test.go
git commit -m "feat(agent): MemoryManager composes memprovider backends + builtin digest"
```

---

## Task 3: AuxClient

**Files:**
- Create: `provider/auxiliary.go`
- Create: `provider/auxiliary_test.go`

- [ ] **Step 1: Write the failing test**

Create `provider/auxiliary_test.go`:

```go
package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/message"
)

type countingProvider struct {
	name  string
	fails int
	seen  int
}

func (p *countingProvider) Name() string              { return p.name }
func (p *countingProvider) Available() bool           { return true }
func (p *countingProvider) ModelInfo(string) *ModelInfo {
	return &ModelInfo{ContextLength: 10000}
}
func (p *countingProvider) EstimateTokens(_, _ string) (int, error) { return 1, nil }
func (p *countingProvider) Stream(context.Context, *Request) (Stream, error) {
	return nil, errors.New("not supported")
}
func (p *countingProvider) Complete(_ context.Context, _ *Request) (*Response, error) {
	p.seen++
	if p.fails > 0 {
		p.fails--
		return nil, &Error{Kind: ErrServerError, Provider: p.name, Message: "boom"}
	}
	return &Response{
		Message: message.Message{
			Role: message.RoleAssistant, Content: message.TextContent("ok from " + p.name),
		},
	}, nil
}

func TestAuxClient_AskUsesFirstWorking(t *testing.T) {
	p1 := &countingProvider{name: "openrouter", fails: 1}
	p2 := &countingProvider{name: "nous"}
	ac := NewAuxClient([]Provider{p1, p2})
	text, err := ac.Ask(context.Background(), "summarize", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "nous") {
		t.Errorf("got %q, want response from nous", text)
	}
	if p1.seen != 1 || p2.seen != 1 {
		t.Errorf("seen: p1=%d p2=%d", p1.seen, p2.seen)
	}
}

func TestAuxClient_EmptyChainIsError(t *testing.T) {
	ac := NewAuxClient(nil)
	if _, err := ac.Ask(context.Background(), "x", "y"); err == nil {
		t.Error("expected error on empty chain")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/ -run TestAuxClient -v`
Expected: FAIL — `NewAuxClient` undefined.

- [ ] **Step 3: Implement AuxClient**

Create `provider/auxiliary.go`:

```go
package provider

import (
	"context"
	"errors"

	"github.com/odysseythink/hermind/message"
)

// AuxClient runs lightweight secondary tasks (summarization, vision
// descriptions, cheap re-ranking) across a fallback chain of providers.
// It does not expose tool-use — callers just pass a system prompt and
// a user message and get a text reply back.
type AuxClient struct {
	chain *FallbackChain
}

// NewAuxClient builds a client from an ordered provider list. The
// first available provider is tried first; IsRetryable / Classify
// decide when to try the next one.
func NewAuxClient(providers []Provider) *AuxClient {
	return &AuxClient{chain: NewFallbackChain(providers)}
}

// Ask sends a two-message request and returns the assistant text.
// system is the instruction prompt; user is the content to operate on.
// MaxTokens defaults to 2048 unless the caller passes a Request via
// AskWithRequest.
func (a *AuxClient) Ask(ctx context.Context, system, user string) (string, error) {
	return a.AskWithRequest(ctx, &Request{
		SystemPrompt: system,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(user)},
		},
		MaxTokens: 2048,
	})
}

// AskWithRequest is the escape hatch for callers (e.g. the compressor)
// that want to supply a fully-formed Request — typically to override
// Model or inject long context as a user turn.
func (a *AuxClient) AskWithRequest(ctx context.Context, req *Request) (string, error) {
	if a.chain == nil {
		return "", errors.New("provider: auxiliary chain is empty")
	}
	resp, err := a.chain.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Message.Content.Text(), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/ -run TestAuxClient -v -race`
Expected: PASS (both sub-tests).

- [ ] **Step 5: Commit**

```bash
git add provider/auxiliary.go provider/auxiliary_test.go
git commit -m "feat(provider): AuxClient over FallbackChain for secondary tasks"
```

---

## Task 4: Wire into Engine

**Files:**
- Modify: `agent/engine.go`

- [ ] **Step 1: Survey the existing construction path**

Run: `grep -n "compressor\|auxProvider\|NewCompressor" agent/engine.go`
Expected: find the single spot where the compressor is built.

- [ ] **Step 2: Extend the Engine**

In `agent/engine.go`, add new fields:

```go
type Engine struct {
	// ... existing fields ...
	memory *MemoryManager
	aux    *provider.AuxClient
}
```

In the `NewEngineWithToolsAndAux` constructor, after the compressor wiring:

```go
if aux != nil {
	e.aux = provider.NewAuxClient([]provider.Provider{aux, p}) // aux first, primary as final fallback
}
e.memory = NewMemoryManager(nil)
```

(Memory providers are added later by `cli/repl.go` / `cli/web.go` once the config-driven factory is called — that's deliberately out of scope for this plan.)

Expose helpers:

```go
// Memory returns the engine's memory manager so callers can register
// remote providers or observe turns.
func (e *Engine) Memory() *MemoryManager { return e.memory }

// Aux returns the engine's auxiliary client.
func (e *Engine) Aux() *provider.AuxClient { return e.aux }
```

Inside the turn loop, where history size triggers compression (find the call to `e.compressor.Compress`), swap in `CompressIteratively` with a budget:

```go
budget := int(float64(contextLen) * e.config.Compression.TargetRatio)
history, passes, err := e.compressor.CompressIteratively(ctx, history, budget)
if err != nil {
	return nil, fmt.Errorf("engine: compress: %w", err)
}
_ = passes // optional: expose as a metric/callback later
```

Also add an `e.memory.ObserveTurn(msg)` call in the per-turn recorder so the digest stays fresh.

- [ ] **Step 3: Run the full agent suite to confirm no regressions**

Run: `go test ./agent/...`
Expected: PASS.

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add agent/engine.go
git commit -m "feat(agent): wire MemoryManager + AuxClient + iterative compression into Engine"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - Iterative compression with pass budget ↔ Task 1 ✓
   - `MemoryManager` composing providers + builtin digest ↔ Task 2 ✓
   - `AuxClient` fallback chain ↔ Task 3 ✓
   - Engine integration ↔ Task 4 ✓

2. **Placeholders:** Task 2 Step 3 calls out the `memprovider.Provider` / `memprovider.Turn` shape — adjust if the actual types differ. No TBD content elsewhere.

3. **Type consistency:**
   - `Compressor.CompressIteratively(ctx, history, budget) ([]message.Message, int, error)` stable across Task 1 test + Task 4 engine wiring.
   - `MemoryManager` method set stable across Task 2 and Task 4.
   - `AuxClient.Ask(ctx, system, user) (string, error)` matches the compressor's needs.

4. **Gaps (future work):**
   - Config-driven `memprovider.Provider` loader + REPL wiring (its own plan).
   - Auxiliary-specific retry budgets (today all 4 providers are tried with shared IsRetryable semantics).
   - Metrics / telemetry for `passes` count and memory provider recall latency.

---

## Definition of Done

- `go test ./agent/... ./provider/... -race` all pass.
- `Engine.Memory()` and `Engine.Aux()` return non-nil when the engine is built with an auxiliary provider.
- Compressor's iterative path runs at most `MaxPasses` times and stops early when the budget is met.
