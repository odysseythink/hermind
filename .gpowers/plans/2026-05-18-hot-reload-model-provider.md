# Hot-Reload Model/Provider Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Model and provider configuration changes take effect immediately after Save, without restarting hermind.

**Architecture:** Extract provider-building logic from `cli/engine_deps.go` into a reusable `buildProviders` function. Change `api.ServerOpts.Deps` to `*EngineDeps` and protect it with `atomic.Pointer[EngineDeps]`. Inject a `DepsBuilder` callback from `cli` that rebuilds Provider/AuxProvider on config save. All handlers read deps through `s.deps.Load()`.

**Tech Stack:** Go 1.23+, `sync/atomic`, `pantheonadapter`, chi router

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `cli/provider_builder.go` | **Create** | Shared `buildProviders` function extracted from `BuildEngineDeps`; `RebuildProviderDeps` for hot-reload |
| `cli/engine_deps.go` | **Modify** | Replace inline provider-building with call to `buildProviders` |
| `api/server.go` | **Modify** | `ServerOpts.Deps` → `*EngineDeps`; add `DepsBuilder`; add `deps atomic.Pointer[EngineDeps]`; change all `s.opts.Deps` reads to `s.deps.Load()` |
| `api/handlers_conversation.go` | **Modify** | `s.opts.Deps.Provider` → `s.currentDeps().Provider` |
| `api/handlers_v1_messages.go` | **Modify** | `s.opts.Deps.Provider` → `s.currentDeps().Provider` |
| `api/memory_health.go` | **Modify** | `s.opts.Deps.Presence` → `s.currentDeps().Presence` |
| `api/handlers_config.go` | **Modify** | Reorder save logic: rebuild deps → save → update config; add `rebuildDeps` method |
| `cli/web.go` | **Modify** | Pass `&deps` and inject `DepsBuilder` callback |
| `cmd/go-desktop-interface/init.go` | **Modify** | Pass `&deps` and inject `DepsBuilder` callback |
| `api/handlers_config_test.go` | **Modify** | Add tests for builder success and builder failure paths |

---

## Task 1: Extract shared provider-building logic

**Files:**
- Create: `cli/provider_builder.go`
- Modify: `cli/engine_deps.go:65-133`

- [ ] **Step 1: Create `cli/provider_builder.go`**

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/pantheonadapter"
	"github.com/odysseythink/pantheon/core"
)

// buildProviders constructs primary and auxiliary LanguageModel instances from
// cfg. It mirrors the provider-building logic in BuildEngineDeps so that
// RebuildProviderDeps can reuse the same code paths.
//
// When the primary provider has no API key, primary is nil and err is nil
// (degraded mode). An error is returned only when the configuration is
// syntactically invalid (unknown provider, bad base URL, etc.).
func buildProviders(ctx context.Context, cfg *config.Config) (primary, aux core.LanguageModel, err error) {
	primaryName := cfg.Model
	if idx := strings.Index(cfg.Model, "/"); idx >= 0 {
		primaryName = cfg.Model[:idx]
	}

	pCfg, ok := cfg.Providers[primaryName]
	if !ok {
		pCfg = config.ProviderConfig{Provider: primaryName}
	}
	if pCfg.Provider == "" {
		pCfg.Provider = primaryName
	}
	if primaryName == "anthropic" && pCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			pCfg.APIKey = envKey
		}
	}
	if pCfg.Model == "" {
		pCfg.Model = defaultModelFromString(cfg.Model)
	}

	if pCfg.APIKey != "" {
		primary, err = pantheonadapter.BuildPrimaryModel(ctx, pCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("build primary provider: %w", err)
		}
	}

	fallbackCfgs := cfg.FallbackProviders
	if primary != nil && len(fallbackCfgs) > 0 {
		fbModel, fbErr := pantheonadapter.BuildFallbackModel(ctx, pCfg, fallbackCfgs)
		if fbErr == nil {
			primary = fbModel
		}
	}

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
		aux, _ = pantheonadapter.BuildModel(ctx, auxCfg)
	}
	if aux == nil {
		aux = primary
	}

	return primary, aux, nil
}

// RebuildProviderDeps creates a new EngineDeps with updated Provider and
// AuxProvider based on cfg, copying all other fields from current.
func RebuildProviderDeps(ctx context.Context, cfg *config.Config, current *api.EngineDeps) (*api.EngineDeps, error) {
	primary, aux, err := buildProviders(ctx, cfg)
	if err != nil {
		return nil, err
	}
	newDeps := *current
	newDeps.Provider = primary
	newDeps.AuxProvider = aux
	return &newDeps, nil
}
```

- [ ] **Step 2: Replace provider-building in `BuildEngineDeps`**

In `cli/engine_deps.go`, replace lines 73-133 (from `var p core.LanguageModel` through the auxiliary provider block) with:

```go
	p, auxModel, err := buildProviders(ctx, app.Config)
	if err != nil {
		return api.EngineDeps{}, cleanup, err
	}
	if p == nil {
		primaryName := app.Config.Model
		if idx := strings.Index(app.Config.Model, "/"); idx >= 0 {
			primaryName = app.Config.Model[:idx]
		}
		fmt.Fprintf(os.Stderr, "%v: provider %q. Set api_key in <instance>/config.yaml or ANTHROPIC_API_KEY env var\n", errMissingAPIKey, primaryName)
		fmt.Fprintln(os.Stderr, "hermind: starting in degraded mode. Chat will fail until you configure a provider.")
	}
```

- [ ] **Step 3: Verify Go compiles**

Run: `cd /d/workspace/go_work/hermind && go build ./cli/...`
Expected: clean compile, no errors.

- [ ] **Step 4: Commit**

```bash
git add cli/provider_builder.go cli/engine_deps.go
git commit -m "refactor(cli): extract buildProviders for reuse in hot-reload"
```

---

## Task 2: Add atomic deps storage to api.Server

**Files:**
- Modify: `api/server.go:36-373`

- [ ] **Step 1: Change `ServerOpts.Deps` to pointer and add `DepsBuilder`**

In `api/server.go`, replace the `Deps` field (line 95-99):

```go
	// Deps is the pre-built Engine dependency bundle. Callers
	// (cli/web.go) fill this via cli.BuildEngineDeps. Required for the
	// POST /api/conversation/messages endpoint; zero-value leaves the
	// endpoint returning 503.
	Deps *EngineDeps

	// DepsBuilder rebuilds the provider-dependent parts of EngineDeps
	// from a new config. When non-nil, handleConfigPut invokes it after
	// parsing the payload so model/provider changes take effect without
	// a server restart.
	DepsBuilder func(ctx context.Context, cfg *config.Config, current *EngineDeps) (*EngineDeps, error)
```

- [ ] **Step 2: Add `deps` field to `Server` struct**

After `idle *idle.IdleConsolidator` (line 113), add:

```go
	// deps holds the current EngineDeps and is swapped atomically when
	// the configuration is hot-reloaded.
	deps atomic.Pointer[EngineDeps]
```

- [ ] **Step 3: Update `NewServer` to initialize atomic deps**

Replace the body of `NewServer` (lines 117-128):

```go
func NewServer(opts *ServerOpts) (*Server, error) {
	if opts == nil || opts.Config == nil {
		return nil, fmt.Errorf("api: ServerOpts.Config is required")
	}
	if opts.Deps == nil {
		opts.Deps = &EngineDeps{}
	}
	streams := opts.Streams
	if streams == nil {
		streams = NewMemoryStreamHub()
	}
	s := &Server{opts: opts, bootedAt: time.Now(), streams: streams}
	s.deps.Store(opts.Deps)
	s.router = s.buildRouter()
	return s, nil
}
```

- [ ] **Step 4: Add `currentDeps` helper**

After the `SetIdleConsolidator` method (line 140-141), add:

```go
// currentDeps returns the live EngineDeps. Callers must not mutate the
// returned value.
func (s *Server) currentDeps() *EngineDeps {
	return s.deps.Load()
}
```

- [ ] **Step 5: Replace `s.opts.Deps` reads in `buildRouter`**

In the middleware (line 156-163):

```go
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if h := s.currentDeps().HTTPIdle; h != nil {
				h.NoteActivity()
			}
			next.ServeHTTP(w, req)
		})
	})
```

- [ ] **Step 6: Replace `s.opts.Deps` reads in `RunTurn`**

Replace lines 268-330. All `s.opts.Deps.Xxx` become `s.currentDeps().Xxx`:

```go
func (s *Server) RunTurn(ctx context.Context, userMessage string) (string, error) {
	deps := s.currentDeps()
	if deps.Provider == nil {
		return "", errors.New("provider not configured")
	}
	s.runMu.Lock()
	if s.runCancel != nil {
		s.runMu.Unlock()
		return "", errors.New("another turn is in flight")
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.runCancel = cancel
	s.runMu.Unlock()

	defer func() {
		s.runMu.Lock()
		s.runCancel = nil
		s.runMu.Unlock()
		cancel()
	}()

	eng := agent.NewEngineWithToolsAndAux(
		deps.Provider, deps.AuxProvider, deps.Storage,
		deps.ToolReg, deps.AgentCfg, deps.Platform,
	)
	if deps.MemProvider != nil {
		eng.Memory().AddProvider(deps.MemProvider)
		if r, ok := deps.MemProvider.(memprovider.Recaller); ok {
			mc := s.opts.Config.Memory.MetaClaw
			memK := mc.InjectCount
			if memK <= 0 {
				memK = 3
			}
			eng.SetActiveMemoriesProvider(func(ctx context.Context, userMsg string) []memprovider.InjectedMemory {
				out, _ := r.Recall(ctx, userMsg, memK)
				return out
			})
			if mc.BufferEvery > 0 {
				eng.SetBufferEvery(mc.BufferEvery)
			}
			if mc.SynergyTokenBudget > 0 {
				eng.SetSynergyBudget(agent.SynergyBudget{
					TokenBudget:  mc.SynergyTokenBudget,
					SkillRatio:   mc.SynergySkillRatio,
					DedupJaccard: 0.5,
				})
			}
			if mc.JudgeEnabled && deps.AuxProvider != nil {
				eng.SetConversationJudge(agent.NewLLMJudge(deps.AuxProvider))
			}
		}
	}
	wireEngineToHub(eng, s.streams)

	if deps.SkillsEvolver != nil {
		eng.SetSkillsEvolver(deps.SkillsEvolver)
	}
	if deps.SkillsRetriever != nil {
		injectCount := s.opts.Config.Skills.InjectCount
		if injectCount <= 0 {
			injectCount = 3
		}
		ret := deps.SkillsRetriever
		eng.SetActiveSkillsProvider(func(userMsg string) []agent.ActiveSkill {
			snippets, _ := ret.Retrieve(runCtx, userMsg, injectCount)
			return snippetsToActiveSkills(snippets)
		})
	}

	result, err := eng.RunConversation(runCtx, &agent.RunOptions{
		UserMessage: userMessage,
	})
	if err != nil {
		s.streams.Publish(StreamEvent{
			Type: EventTypeError,
			Data: map[string]any{"message": err.Error()},
		})
		return "", err
	}
	s.streams.Publish(StreamEvent{Type: EventTypeDone})
	return result.Response.Text(), nil
}
```

- [ ] **Step 7: Verify Go compiles**

Run: `go build ./api/...`
Expected: clean compile. May fail because handlers still use `s.opts.Deps` — that's OK, fixed in Task 3.

- [ ] **Step 8: Commit**

```bash
git add api/server.go
git commit -m "feat(api): add atomic deps storage and currentDeps helper"
```

---

## Task 3: Update remaining handlers to use atomic deps

**Files:**
- Modify: `api/handlers_conversation.go:120`
- Modify: `api/handlers_conversation.go:145-147`
- Modify: `api/handlers_v1_messages.go:26`
- Modify: `api/memory_health.go:32`

- [ ] **Step 1: Update `api/handlers_conversation.go`**

Line 120:
```go
	if s.currentDeps().Provider == nil {
```

Lines 145-147:
```go
	eng := agent.NewEngineWithToolsAndAux(
		s.currentDeps().Provider, s.currentDeps().AuxProvider, s.currentDeps().Storage,
		s.currentDeps().ToolReg, s.currentDeps().AgentCfg, s.currentDeps().Platform,
	)
```

- [ ] **Step 2: Update `api/handlers_v1_messages.go`**

Line 26:
```go
	prov := s.currentDeps().Provider
```

- [ ] **Step 3: Update `api/memory_health.go`**

Line 32:
```go
	if p := s.currentDeps().Presence; p != nil {
```

- [ ] **Step 4: Verify Go compiles**

Run: `go build ./api/...`
Expected: clean compile.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_conversation.go api/handlers_v1_messages.go api/memory_health.go
git commit -m "feat(api): read deps through atomic pointer in all handlers"
```

---

## Task 4: Wire rebuild into handleConfigPut

**Files:**
- Modify: `api/handlers_config.go:148-181`

- [ ] **Step 1: Add `rebuildDeps` method**

Add after `handleConfigPut` (after line 181):

```go
// rebuildDeps calls the configured DepsBuilder with cfg and swaps the
// active deps atomically. If no builder is configured it is a no-op.
func (s *Server) rebuildDeps(ctx context.Context, cfg *config.Config) error {
	if s.opts.DepsBuilder == nil {
		return nil
	}
	current := s.deps.Load()
	newDeps, err := s.opts.DepsBuilder(ctx, cfg, current)
	if err != nil {
		return err
	}
	s.deps.Store(newDeps)
	return nil
}
```

- [ ] **Step 2: Reorder handleConfigPut logic**

Replace the second half of `handleConfigPut` (line 174 onwards):

```go
	preserveSecrets(&updated, s.opts.Config)

	if err := s.rebuildDeps(r.Context(), &updated); err != nil {
		http.Error(w, "invalid provider config: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := config.SaveToPath(s.opts.ConfigPath, &updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	*s.opts.Config = updated
	writeJSON(w, OKResponse{OK: true})
```

- [ ] **Step 3: Verify Go compiles**

Run: `go build ./api/...`
Expected: clean compile.

- [ ] **Step 4: Commit**

```bash
git add api/handlers_config.go
git commit -m "feat(api): rebuild deps before saving config in handleConfigPut"
```

---

## Task 5: Inject DepsBuilder from cli and desktop

**Files:**
- Modify: `cli/web.go:54-66`
- Modify: `cmd/go-desktop-interface/init.go:36-45`

- [ ] **Step 1: Update `cli/web.go`**

Replace the `api.NewServer` call (lines 54-66):

```go
	srv, err := api.NewServer(&api.ServerOpts{
		Config:       app.Config,
		ConfigPath:   app.ConfigPath,
		InstanceRoot: app.InstanceRoot,
		Storage:      app.Storage,
		Version:      Version,
		Streams:      streams,
		Deps:         &deps,
		DepsBuilder: func(ctx context.Context, cfg *config.Config, current *api.EngineDeps) (*api.EngineDeps, error) {
			return RebuildProviderDeps(ctx, cfg, current)
		},
	})
```

- [ ] **Step 2: Update `cmd/go-desktop-interface/init.go`**

Replace the `api.NewServer` call (lines 36-45):

```go
	srv, err := api.NewServer(&api.ServerOpts{
		Config:       app.Config,
		ConfigPath:   app.ConfigPath,
		InstanceRoot: app.InstanceRoot,
		Storage:      app.Storage,
		Version:      cli.Version,
		Streams:      streams,
		Deps:         &deps,
		DepsBuilder: func(ctx context.Context, cfg *config.Config, current *api.EngineDeps) (*api.EngineDeps, error) {
			return cli.RebuildProviderDeps(ctx, cfg, current)
		},
	})
```

- [ ] **Step 3: Verify Go compiles**

Run: `go build ./cli/... ./cmd/...`
Expected: clean compile.

- [ ] **Step 4: Commit**

```bash
git add cli/web.go cmd/go-desktop-interface/init.go
git commit -m "feat(cli): inject DepsBuilder for hot-reload model/provider"
```

---

## Task 6: Add api tests for hot-reload

**Files:**
- Modify: `api/handlers_config_test.go`

- [ ] **Step 1: Add test for successful deps rebuild**

Append to `api/handlers_config_test.go`:

```go
func TestHandleConfigPut_RebuildsDeps(t *testing.T) {
	cfg := &config.Config{}
	cfg.Model = "anthropic/claude-test"
	cfg.Providers = map[string]config.ProviderConfig{
		"anthropic": {Provider: "anthropic", APIKey: "sk-test", Model: "claude-test"},
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")

	builderCalled := false
	builder := func(ctx context.Context, cfg *config.Config, current *EngineDeps) (*EngineDeps, error) {
		builderCalled = true
		newDeps := *current
		// In production this would build real providers; here we just verify
		// the builder is invoked and its result is stored.
		newDeps.Provider = &stubLM{name: "rebuilt"}
		return &newDeps, nil
	}

	srv, err := NewServer(&ServerOpts{
		Config:      cfg,
		ConfigPath:  path,
		Deps:        &EngineDeps{},
		DepsBuilder: builder,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	putBody := strings.NewReader(`{"config":{"model":"openai/gpt-test","providers":{"openai":{"provider":"openai","api_key":"sk-new","model":"gpt-test"}}}}`)
	req := httptest.NewRequest("PUT", "/api/config", putBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", w.Code, w.Body.String())
	}
	if !builderCalled {
		t.Error("DepsBuilder was not called")
	}
	if srv.currentDeps().Provider == nil {
		t.Error("Provider was not rebuilt after config put")
	}
}

func TestHandleConfigPut_BuilderError_AbortsSave(t *testing.T) {
	cfg := &config.Config{}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")

	builder := func(ctx context.Context, cfg *config.Config, current *EngineDeps) (*EngineDeps, error) {
		return nil, errors.New("bad provider config")
	}

	srv, err := NewServer(&ServerOpts{
		Config:      cfg,
		ConfigPath:  path,
		Deps:        &EngineDeps{},
		DepsBuilder: builder,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	putBody := strings.NewReader(`{"config":{"model":"anthropic/claude-test","providers":{"anthropic":{"provider":"anthropic","api_key":"sk-test"}}}}`)
	req := httptest.NewRequest("PUT", "/api/config", putBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	// Config file should NOT have been written.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("config file was saved despite builder error")
	}
}

// stubLM is a minimal core.LanguageModel for tests.
type stubLM struct{ name string }

func (m *stubLM) Provider() string { return "stub" }
func (m *stubLM) Model() string    { return m.name }
func (m *stubLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, errors.New("stub")
}
func (m *stubLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, errors.New("stub")
}
func (m *stubLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("stub")
}
```

- [ ] **Step 2: Add imports to test file**

Add to the imports block in `api/handlers_config_test.go`:

```go
	"context"
	"errors"
	"os"

	"github.com/odysseythink/pantheon/core"
```

- [ ] **Step 3: Run new tests**

Run: `go test ./api/ -run TestHandleConfigPut_RebuildsDeps -v`
Expected: PASS

Run: `go test ./api/ -run TestHandleConfigPut_BuilderError_AbortsSave -v`
Expected: PASS

- [ ] **Step 4: Run full api test suite**

Run: `go test ./api/...`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config_test.go
git commit -m "test(api): verify hot-reload builder success and failure paths"
```

---

## Task 7: Full regression test

**Files:** None (verification only)

- [ ] **Step 1: Run all Go tests**

Run: `go test ./...`
Expected: all packages pass.

- [ ] **Step 2: Build the binary**

Run: `go build ./cmd/hermind`
Expected: clean build.

- [ ] **Step 3: Build desktop interface**

Run: `go build ./cmd/go-desktop-interface`
Expected: clean build.

- [ ] **Step 4: Commit (if any fixes were needed)**

```bash
git commit -m "fix: resolve regressions from hot-reload model/provider" || echo "no fixes needed"
```

---

## Self-Review

### Spec coverage check

| Design requirement | Task |
|---|---|
| `ServerOpts.Deps` → pointer | Task 2, Step 1 |
| `atomic.Pointer[EngineDeps]` | Task 2, Step 2-3 |
| `DepsBuilder` callback | Task 2, Step 1; Task 5 |
| All handlers read via `s.deps.Load()` | Task 2, Step 5-6; Task 3 |
| `handleConfigPut` reorder: rebuild → save → update | Task 4, Step 2 |
| Extract shared provider-building | Task 1 |
| `RebuildProviderDeps` shallow copy + replace Provider/AuxProvider | Task 1, Step 1 |
| Tests for success + failure | Task 6 |
| Full regression | Task 7 |

### Placeholder scan

- No TBD/TODO/fill-in-details found.
- Every step contains concrete code or exact commands.
- Type names consistent: `EngineDeps`, `DepsBuilder`, `currentDeps`, `RebuildProviderDeps`.

### Type consistency check

- `ServerOpts.Deps` is `*EngineDeps` everywhere.
- `Server.deps` is `atomic.Pointer[EngineDeps]`.
- `DepsBuilder` signature matches in definition (`api/server.go`) and injection sites (`cli/web.go`, `cmd/go-desktop-interface/init.go`).
- `RebuildProviderDeps` signature: `(ctx, cfg, current) → (*EngineDeps, error)` — matches `DepsBuilder`.

---

## Execution Handoff

Plan complete and saved to `.gpowers/plans/2026-05-18-hot-reload-model-provider.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints for review

Which approach?
