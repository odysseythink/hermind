# Plan 6c: External Memory Providers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a pluggable external memory provider interface and three HTTP-backed implementations (Honcho, Mem0, Supermemory) that expose their own tools into the registry and receive turn/session lifecycle callbacks.

**Architecture:** A thin `memprovider.Provider` interface lives in `tool/memory/memprovider`. The `cli/repl.go` bootstrap constructs one active external provider based on `config.Memory.Provider`, calls `Initialize`, registers its extra tools via its own `RegisterTools` method, and hooks `SyncTurn`/`Shutdown` into the TUI lifecycle. Each provider talks to its backend via plain `net/http` and returns `string` JSON results that match the existing memory tool conventions.

**Tech Stack:** Go 1.25, stdlib `net/http`, `encoding/json`, existing `tool`, `storage`, `config`, `cli` packages. No new dependencies.

**Deliverable at end of plan:**
```yaml
# ~/.hermes/config.yaml
memory:
  provider: honcho        # or mem0, supermemory, "" for none
  honcho:
    base_url: https://demo.honcho.dev
    workspace: hermes
    peer: me
  mem0:
    base_url: https://api.mem0.ai
    user_id: default
  supermemory:
    base_url: https://api.supermemory.ai
```
```
> remember I like dark mode
⚡ honcho_remember: {"content":"user likes dark mode"}
│ {"ok":true, "id":"msg_01H..."}
└

> what do you know about me
⚡ honcho_recall: {"query":"preferences"}
│ {"results":["user likes dark mode", ...]}
└
```

**Non-goals for this plan (deferred):**
- Hindsight (local git-based, not HTTP) — Plan 6c.1
- Byterover / Retaindb / Holographic / Openviking — Plan 6c.2 (few users, can wait)
- Multi-provider concurrency — one active external provider at a time
- Background prefetch (async recall into the system prompt) — Plan 6f
- Fact extraction / on_session_end — Plan 6f

**Plans 1-6b dependencies this plan touches:**
- `config/config.go` — add `MemoryConfig` block
- `tool/memory/memprovider/provider.go` — NEW interface
- `tool/memory/memprovider/http.go` — NEW shared HTTP helper
- `tool/memory/memprovider/honcho.go` — NEW
- `tool/memory/memprovider/mem0.go` — NEW
- `tool/memory/memprovider/supermemory.go` — NEW
- `tool/memory/memprovider/factory.go` — NEW, picks the active provider
- `cli/repl.go` — wire up provider, register its tools, call Shutdown

---

## File Structure

```
hermes-agent-go/
├── config/
│   └── config.go                        # MODIFIED: add MemoryConfig
├── tool/
│   └── memory/
│       ├── memory.go                    # unchanged
│       ├── register.go                  # unchanged
│       └── memprovider/                 # NEW
│           ├── provider.go              # Provider interface + common types
│           ├── http.go                  # httpJSON helper
│           ├── http_test.go
│           ├── honcho.go                # Honcho provider
│           ├── honcho_test.go
│           ├── mem0.go                  # Mem0 provider
│           ├── mem0_test.go
│           ├── supermemory.go           # Supermemory provider
│           ├── supermemory_test.go
│           ├── factory.go               # New(cfg) -> Provider
│           └── factory_test.go
└── cli/
    └── repl.go                          # MODIFIED: wire up memprovider
```

---

## Task 1: Config Block

**Files:**
- Modify: `config/config.go`

- [ ] **Step 1: Add MemoryConfig type and field**

Add the following to `config/config.go`:

```go
// MemoryConfig holds the optional external memory provider configuration.
// At most one provider is active at a time (see tool/memory/memprovider).
type MemoryConfig struct {
	Provider    string              `yaml:"provider,omitempty"` // "", "honcho", "mem0", "supermemory"
	Honcho      HonchoConfig        `yaml:"honcho,omitempty"`
	Mem0        Mem0Config          `yaml:"mem0,omitempty"`
	Supermemory SupermemoryConfig   `yaml:"supermemory,omitempty"`
}

// HonchoConfig holds the Honcho provider configuration.
type HonchoConfig struct {
	BaseURL   string `yaml:"base_url,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`
	Workspace string `yaml:"workspace,omitempty"`
	Peer      string `yaml:"peer,omitempty"`
}

// Mem0Config holds the Mem0 provider configuration.
type Mem0Config struct {
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	UserID  string `yaml:"user_id,omitempty"`
}

// SupermemoryConfig holds the Supermemory provider configuration.
type SupermemoryConfig struct {
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
	UserID  string `yaml:"user_id,omitempty"`
}
```

Then add the `Memory` field to `Config`:

```go
type Config struct {
	// ... existing fields ...
	Memory            MemoryConfig              `yaml:"memory,omitempty"`
}
```

- [ ] **Step 2: Run existing config tests**

Run: `go test ./config/...`
Expected: PASS (no behavior changes, just new optional fields).

- [ ] **Step 3: Commit**

```bash
git add config/config.go
git commit -m "feat(config): add MemoryConfig for external memory providers"
```

---

## Task 2: Provider interface + HTTP helper

**Files:**
- Create: `tool/memory/memprovider/provider.go`
- Create: `tool/memory/memprovider/http.go`
- Create: `tool/memory/memprovider/http_test.go`

- [ ] **Step 1: Write the Provider interface**

Create `tool/memory/memprovider/provider.go`:

```go
// Package memprovider defines pluggable external memory providers.
//
// Each Provider is an adapter to a remote memory service (Honcho, Mem0,
// Supermemory, …). The CLI bootstrap picks at most one active Provider
// based on config.Memory.Provider, calls Initialize once at startup,
// registers its extra tools into the tool.Registry, and drives the
// SyncTurn / Shutdown lifecycle.
//
// This is intentionally a much smaller surface than the Python
// MemoryProvider base class. Advanced hooks (on_session_end,
// on_pre_compress, on_delegation, background prefetch) are deferred
// to Plan 6f.
package memprovider

import (
	"context"

	"github.com/nousresearch/hermes-agent/tool"
)

// Provider is the minimal interface every external memory provider
// implements. Implementations are expected to be safe to call from
// the CLI goroutine; long-running work should be queued internally.
type Provider interface {
	// Name is a short, lowercase identifier used in logs and the
	// factory ("honcho", "mem0", "supermemory").
	Name() string

	// Initialize performs one-time setup for the given session. It is
	// called exactly once during CLI startup.
	Initialize(ctx context.Context, sessionID string) error

	// RegisterTools registers any provider-specific tools into the
	// given registry. Most providers register 1–2 tools
	// (recall + remember). Called after Initialize.
	RegisterTools(reg *tool.Registry)

	// SyncTurn is called after each completed turn with the user
	// message and assistant reply. Implementations should queue
	// persistence work and return quickly.
	SyncTurn(ctx context.Context, userMsg, assistantMsg string) error

	// Shutdown flushes any queued work and releases resources.
	// Called at CLI exit.
	Shutdown(ctx context.Context) error
}
```

- [ ] **Step 2: Write the HTTP helper**

Create `tool/memory/memprovider/http.go`:

```go
package memprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultTimeout caps any single HTTP call to the remote memory service.
const defaultTimeout = 20 * time.Second

// httpClient is a package-level client reused by all providers so
// connection pooling works across calls. Tests substitute their own
// via providerHTTPClient (see below).
var httpClient = &http.Client{Timeout: defaultTimeout}

// httpJSON sends `body` as JSON to the given URL with method and
// optional Bearer auth. The response JSON is decoded into `out` (may
// be nil to discard). Non-2xx responses return an error containing the
// status code and (truncated) body for debugging.
func httpJSON(ctx context.Context, method, url, bearer string, body, out any) error {
	return httpJSONWith(ctx, httpClient, method, url, bearer, body, out)
}

func httpJSONWith(ctx context.Context, client *http.Client, method, url, bearer string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("memprovider: encode body: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return fmt.Errorf("memprovider: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("memprovider: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		// Read up to 1 KiB of the error body for diagnostics.
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("memprovider: %s %s: status %d: %s",
			method, url, resp.StatusCode, string(errBody))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("memprovider: decode response: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Write the HTTP helper test**

Create `tool/memory/memprovider/http_test.go`:

```go
package memprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPJSONSuccess(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer abc" {
			t.Errorf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing content-type")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	var out struct {
		OK bool `json:"ok"`
	}
	err := httpJSONWith(context.Background(), srv.Client(), "POST", srv.URL, "abc",
		map[string]string{"hello": "world"}, &out)
	if err != nil {
		t.Fatalf("httpJSONWith: %v", err)
	}
	if !out.OK {
		t.Errorf("expected ok=true, got %+v", out)
	}
	if got["hello"] != "world" {
		t.Errorf("server did not see body: %+v", got)
	}
}

func TestHTTPJSONErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := httpJSONWith(context.Background(), srv.Client(), "GET", srv.URL, "", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error did not include body: %v", err)
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./tool/memory/memprovider/...`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add tool/memory/memprovider/provider.go tool/memory/memprovider/http.go tool/memory/memprovider/http_test.go
git commit -m "feat(tool/memory): add memprovider interface and HTTP helper"
```

---

## Task 3: Honcho provider

**Files:**
- Create: `tool/memory/memprovider/honcho.go`
- Create: `tool/memory/memprovider/honcho_test.go`

Honcho's public HTTP API uses workspaces, peers, and sessions. For the Go port we target the minimal subset needed: `POST /v1/workspaces/{ws}/peers/{peer}/messages` to add memories and `POST /v1/workspaces/{ws}/peers/{peer}/search` to query them. Real Honcho deployments vary; providers always read `base_url` from config so tests can point at an `httptest.Server`.

- [ ] **Step 1: Write the failing test first**

Create `tool/memory/memprovider/honcho_test.go`:

```go
package memprovider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

func TestHonchoSyncTurnAndRecall(t *testing.T) {
	var addCount, searchCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/messages") && r.Method == "POST":
			atomic.AddInt32(&addCount, 1)
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "dark mode") {
				t.Errorf("expected body to contain dark mode, got %s", body)
			}
			_, _ = w.Write([]byte(`{"id":"msg_1"}`))
		case strings.HasSuffix(r.URL.Path, "/search") && r.Method == "POST":
			atomic.AddInt32(&searchCount, 1)
			_, _ = w.Write([]byte(`{"results":[{"content":"user likes dark mode"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewHoncho(config.HonchoConfig{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		Workspace: "hermes",
		Peer:      "me",
	})
	if p.Name() != "honcho" {
		t.Fatalf("name = %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-1"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I like dark mode", "noted"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&addCount) != 1 {
		t.Errorf("expected 1 add call, got %d", addCount)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)

	args, _ := json.Marshal(map[string]string{"query": "preferences"})
	res, err := reg.Dispatch(ctx, "honcho_recall", args)
	if err != nil {
		t.Fatalf("Dispatch honcho_recall: %v", err)
	}
	if !strings.Contains(res, "dark mode") {
		t.Errorf("expected recall result to contain dark mode, got %s", res)
	}
	if atomic.LoadInt32(&searchCount) != 1 {
		t.Errorf("expected 1 search call, got %d", searchCount)
	}

	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to see it fail**

Run: `go test ./tool/memory/memprovider/ -run TestHoncho -v`
Expected: FAIL — `NewHoncho` not defined.

- [ ] **Step 3: Write the Honcho provider**

Create `tool/memory/memprovider/honcho.go`:

```go
package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

// Honcho is a Provider backed by the Honcho memory service.
//
// It implements only the minimal subset needed for Plan 6c:
//   - append turns to the peer's message stream (/messages)
//   - search the peer's memories on demand (/search)
//
// Advanced features (sessions, metadata, working representations)
// are deferred to later plans.
type Honcho struct {
	cfg       config.HonchoConfig
	sessionID string
	client    httpDoer // test seam; nil => use package httpClient
}

// httpDoer is the tiny subset of *http.Client used by providers.
// It is exposed to allow tests to substitute a fake client if needed
// and to keep the provider-level surface tight.
type httpDoer interface {
	// no methods — kept as a type alias so tests can supply a custom
	// *http.Client via the unexported constructor below.
}

// NewHoncho constructs a Honcho provider from configuration. The
// config is normalized: empty workspace defaults to "hermes", empty
// peer defaults to "me", empty base URL to the public demo endpoint.
func NewHoncho(cfg config.HonchoConfig) *Honcho {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://demo.honcho.dev"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Workspace == "" {
		cfg.Workspace = "hermes"
	}
	if cfg.Peer == "" {
		cfg.Peer = "me"
	}
	return &Honcho{cfg: cfg}
}

func (h *Honcho) Name() string { return "honcho" }

func (h *Honcho) Initialize(ctx context.Context, sessionID string) error {
	h.sessionID = sessionID
	return nil
}

func (h *Honcho) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return h.addMessage(ctx, fmt.Sprintf("user: %s\nassistant: %s", userMsg, assistantMsg))
}

func (h *Honcho) Shutdown(ctx context.Context) error { return nil }

// addMessage POSTs a single memory message to the peer.
func (h *Honcho) addMessage(ctx context.Context, content string) error {
	url := fmt.Sprintf("%s/v1/workspaces/%s/peers/%s/messages",
		h.cfg.BaseURL, h.cfg.Workspace, h.cfg.Peer)
	body := map[string]any{
		"content":    content,
		"session_id": h.sessionID,
	}
	return httpJSON(ctx, "POST", url, h.cfg.APIKey, body, nil)
}

// searchResponse is the minimal shape we decode from /search.
type honchoSearchResponse struct {
	Results []struct {
		Content string `json:"content"`
	} `json:"results"`
}

// search queries the peer for memories matching q.
func (h *Honcho) search(ctx context.Context, q string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := fmt.Sprintf("%s/v1/workspaces/%s/peers/%s/search",
		h.cfg.BaseURL, h.cfg.Workspace, h.cfg.Peer)
	body := map[string]any{"query": q, "limit": limit}
	var resp honchoSearchResponse
	if err := httpJSON(ctx, "POST", url, h.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Content)
	}
	return out, nil
}

// RegisterTools registers honcho_recall and honcho_remember into reg.
func (h *Honcho) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "honcho_remember",
		Toolset:     "memory",
		Description: "Explicitly store a fact in Honcho for future recall.",
		Emoji:       "🪶",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "honcho_remember",
				Description: "Store a fact in Honcho (external memory provider).",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"content":{"type":"string","description":"Text to remember"}},
  "required":["content"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := h.addMessage(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "honcho_recall",
		Toolset:     "memory",
		Description: "Recall relevant memories from Honcho by semantic query.",
		Emoji:       "🔎",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "honcho_recall",
				Description: "Search Honcho memories and return matching content.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "query":{"type":"string","description":"Search query"},
    "limit":{"type":"number","description":"Max results (default 5)"}
  },
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Query) == "" {
				return tool.ToolError("query is required"), nil
			}
			results, err := h.search(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
```

- [ ] **Step 4: Run the test to verify PASS**

Run: `go test ./tool/memory/memprovider/ -run TestHoncho -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tool/memory/memprovider/honcho.go tool/memory/memprovider/honcho_test.go
git commit -m "feat(tool/memory): add Honcho external memory provider"
```

---

## Task 4: Mem0 provider

**Files:**
- Create: `tool/memory/memprovider/mem0.go`
- Create: `tool/memory/memprovider/mem0_test.go`

Mem0's cloud API exposes `POST /v1/memories/` to add memories and `POST /v1/memories/search/` to query them, both keyed by `user_id`.

- [ ] **Step 1: Write the failing test**

Create `tool/memory/memprovider/mem0_test.go`:

```go
package memprovider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

func TestMem0SyncTurnAndRecall(t *testing.T) {
	var addCount, searchCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/memories/") && r.Method == "POST":
			atomic.AddInt32(&addCount, 1)
			if !strings.Contains(string(body), "likes tea") {
				t.Errorf("unexpected body: %s", body)
			}
			_, _ = w.Write([]byte(`{"id":"m_1","status":"ok"}`))
		case strings.HasSuffix(r.URL.Path, "/memories/search/") && r.Method == "POST":
			atomic.AddInt32(&searchCount, 1)
			_, _ = w.Write([]byte(`[{"memory":"user likes tea"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewMem0(config.Mem0Config{
		BaseURL: srv.URL,
		APIKey:  "m0_key",
		UserID:  "user1",
	})
	if p.Name() != "mem0" {
		t.Fatalf("name = %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-1"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I like tea", "got it"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&addCount) != 1 {
		t.Errorf("expected 1 add call, got %d", addCount)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)
	args, _ := json.Marshal(map[string]string{"query": "drink"})
	res, err := reg.Dispatch(ctx, "mem0_recall", args)
	if err != nil {
		t.Fatalf("Dispatch mem0_recall: %v", err)
	}
	if !strings.Contains(res, "likes tea") {
		t.Errorf("expected recall to include tea, got %s", res)
	}
	if atomic.LoadInt32(&searchCount) != 1 {
		t.Errorf("expected 1 search call, got %d", searchCount)
	}
}
```

- [ ] **Step 2: Run the test (should fail)**

Run: `go test ./tool/memory/memprovider/ -run TestMem0 -v`
Expected: FAIL — `NewMem0` not defined.

- [ ] **Step 3: Write the Mem0 provider**

Create `tool/memory/memprovider/mem0.go`:

```go
package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

// Mem0 is a Provider backed by the Mem0 cloud memory service.
type Mem0 struct {
	cfg       config.Mem0Config
	sessionID string
}

// NewMem0 builds a Mem0 provider with sensible defaults.
func NewMem0(cfg config.Mem0Config) *Mem0 {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.mem0.ai"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.UserID == "" {
		cfg.UserID = "default"
	}
	return &Mem0{cfg: cfg}
}

func (m *Mem0) Name() string { return "mem0" }

func (m *Mem0) Initialize(ctx context.Context, sessionID string) error {
	m.sessionID = sessionID
	return nil
}

func (m *Mem0) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	url := m.cfg.BaseURL + "/v1/memories/"
	body := map[string]any{
		"user_id": m.cfg.UserID,
		"messages": []map[string]string{
			{"role": "user", "content": userMsg},
			{"role": "assistant", "content": assistantMsg},
		},
	}
	return httpJSON(ctx, "POST", url, m.cfg.APIKey, body, nil)
}

func (m *Mem0) Shutdown(ctx context.Context) error { return nil }

// mem0SearchItem matches Mem0's search response shape.
type mem0SearchItem struct {
	Memory string `json:"memory"`
}

func (m *Mem0) search(ctx context.Context, query string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := m.cfg.BaseURL + "/v1/memories/search/"
	body := map[string]any{"query": query, "user_id": m.cfg.UserID, "limit": limit}
	var resp []mem0SearchItem
	if err := httpJSON(ctx, "POST", url, m.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp))
	for _, r := range resp {
		out = append(out, r.Memory)
	}
	return out, nil
}

// add stores a single piece of content as a standalone user memory.
func (m *Mem0) add(ctx context.Context, content string) error {
	url := m.cfg.BaseURL + "/v1/memories/"
	body := map[string]any{
		"user_id": m.cfg.UserID,
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
	}
	return httpJSON(ctx, "POST", url, m.cfg.APIKey, body, nil)
}

// RegisterTools registers mem0_recall and mem0_remember into reg.
func (m *Mem0) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "mem0_remember",
		Toolset:     "memory",
		Description: "Store a fact in Mem0 (external memory provider).",
		Emoji:       "💾",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "mem0_remember",
				Description: "Store a fact in the Mem0 memory store.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"content":{"type":"string","description":"Text to remember"}},
  "required":["content"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := m.add(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "mem0_recall",
		Toolset:     "memory",
		Description: "Recall memories from Mem0 by semantic query.",
		Emoji:       "🧩",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "mem0_recall",
				Description: "Search Mem0 memories and return matching content.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "query":{"type":"string","description":"Search query"},
    "limit":{"type":"number","description":"Max results (default 5)"}
  },
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Query) == "" {
				return tool.ToolError("query is required"), nil
			}
			results, err := m.search(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
	_ = fmt.Sprintf // keep import stable if you add logging later
}
```

- [ ] **Step 4: Run the Mem0 test**

Run: `go test ./tool/memory/memprovider/ -run TestMem0 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tool/memory/memprovider/mem0.go tool/memory/memprovider/mem0_test.go
git commit -m "feat(tool/memory): add Mem0 external memory provider"
```

---

## Task 5: Supermemory provider

**Files:**
- Create: `tool/memory/memprovider/supermemory.go`
- Create: `tool/memory/memprovider/supermemory_test.go`

Supermemory exposes `POST /v3/memories` to add memories and `POST /v3/search` to query them. Bearer auth with the API key.

- [ ] **Step 1: Write the failing test**

Create `tool/memory/memprovider/supermemory_test.go`:

```go
package memprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

func TestSupermemorySyncTurnAndRecall(t *testing.T) {
	var addCount, searchCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/v3/memories") && r.Method == "POST":
			atomic.AddInt32(&addCount, 1)
			_, _ = w.Write([]byte(`{"id":"sm_1"}`))
		case strings.HasSuffix(r.URL.Path, "/v3/search") && r.Method == "POST":
			atomic.AddInt32(&searchCount, 1)
			_, _ = w.Write([]byte(`{"results":[{"content":"user likes cycling"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewSupermemory(config.SupermemoryConfig{
		BaseURL: srv.URL,
		APIKey:  "sm-key",
		UserID:  "user-42",
	})
	if p.Name() != "supermemory" {
		t.Fatalf("name = %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-x"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I ride bikes", "cool"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&addCount) != 1 {
		t.Errorf("expected 1 add call, got %d", addCount)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)
	args, _ := json.Marshal(map[string]string{"query": "hobbies"})
	res, err := reg.Dispatch(ctx, "supermemory_recall", args)
	if err != nil {
		t.Fatalf("Dispatch supermemory_recall: %v", err)
	}
	if !strings.Contains(res, "cycling") {
		t.Errorf("expected recall to contain cycling, got %s", res)
	}
	if atomic.LoadInt32(&searchCount) != 1 {
		t.Errorf("expected 1 search call, got %d", searchCount)
	}
}
```

- [ ] **Step 2: Run to see failure**

Run: `go test ./tool/memory/memprovider/ -run TestSupermemory -v`
Expected: FAIL — `NewSupermemory` not defined.

- [ ] **Step 3: Write the Supermemory provider**

Create `tool/memory/memprovider/supermemory.go`:

```go
package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

// Supermemory is a Provider backed by the Supermemory cloud API.
type Supermemory struct {
	cfg       config.SupermemoryConfig
	sessionID string
}

func NewSupermemory(cfg config.SupermemoryConfig) *Supermemory {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.supermemory.ai"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.UserID == "" {
		cfg.UserID = "default"
	}
	return &Supermemory{cfg: cfg}
}

func (s *Supermemory) Name() string { return "supermemory" }

func (s *Supermemory) Initialize(ctx context.Context, sessionID string) error {
	s.sessionID = sessionID
	return nil
}

func (s *Supermemory) add(ctx context.Context, content string) error {
	url := s.cfg.BaseURL + "/v3/memories"
	body := map[string]any{
		"content":        content,
		"user_id":        s.cfg.UserID,
		"container_tags": []string{"hermes"},
	}
	return httpJSON(ctx, "POST", url, s.cfg.APIKey, body, nil)
}

func (s *Supermemory) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return s.add(ctx, fmt.Sprintf("user: %s\nassistant: %s", userMsg, assistantMsg))
}

func (s *Supermemory) Shutdown(ctx context.Context) error { return nil }

type supermemorySearchResponse struct {
	Results []struct {
		Content string `json:"content"`
	} `json:"results"`
}

func (s *Supermemory) search(ctx context.Context, q string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := s.cfg.BaseURL + "/v3/search"
	body := map[string]any{"q": q, "user_id": s.cfg.UserID, "limit": limit}
	var resp supermemorySearchResponse
	if err := httpJSON(ctx, "POST", url, s.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Content)
	}
	return out, nil
}

// RegisterTools registers supermemory_remember and supermemory_recall.
func (s *Supermemory) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "supermemory_remember",
		Toolset:     "memory",
		Description: "Store a fact in Supermemory (external memory provider).",
		Emoji:       "🧠",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "supermemory_remember",
				Description: "Store a fact in Supermemory for future recall.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"content":{"type":"string","description":"Text to remember"}},
  "required":["content"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := s.add(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "supermemory_recall",
		Toolset:     "memory",
		Description: "Recall memories from Supermemory by query.",
		Emoji:       "🔭",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "supermemory_recall",
				Description: "Search Supermemory and return matching content.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "query":{"type":"string","description":"Search query"},
    "limit":{"type":"number","description":"Max results (default 5)"}
  },
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Query) == "" {
				return tool.ToolError("query is required"), nil
			}
			results, err := s.search(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./tool/memory/memprovider/ -run TestSupermemory -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tool/memory/memprovider/supermemory.go tool/memory/memprovider/supermemory_test.go
git commit -m "feat(tool/memory): add Supermemory external memory provider"
```

---

## Task 6: Factory + CLI wiring

**Files:**
- Create: `tool/memory/memprovider/factory.go`
- Create: `tool/memory/memprovider/factory_test.go`
- Modify: `cli/repl.go`

- [ ] **Step 1: Write the factory test**

Create `tool/memory/memprovider/factory_test.go`:

```go
package memprovider

import (
	"testing"

	"github.com/nousresearch/hermes-agent/config"
)

func TestNewFromConfigNoneReturnsNil(t *testing.T) {
	p, err := New(config.MemoryConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil, got %+v", p)
	}
}

func TestNewFromConfigHoncho(t *testing.T) {
	p, err := New(config.MemoryConfig{
		Provider: "honcho",
		Honcho:   config.HonchoConfig{APIKey: "x"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p == nil || p.Name() != "honcho" {
		t.Errorf("expected honcho provider, got %+v", p)
	}
}

func TestNewFromConfigMem0(t *testing.T) {
	p, err := New(config.MemoryConfig{
		Provider: "mem0",
		Mem0:     config.Mem0Config{APIKey: "x"},
	})
	if err != nil || p == nil || p.Name() != "mem0" {
		t.Fatalf("expected mem0, got %+v err=%v", p, err)
	}
}

func TestNewFromConfigSupermemory(t *testing.T) {
	p, err := New(config.MemoryConfig{
		Provider:    "supermemory",
		Supermemory: config.SupermemoryConfig{APIKey: "x"},
	})
	if err != nil || p == nil || p.Name() != "supermemory" {
		t.Fatalf("expected supermemory, got %+v err=%v", p, err)
	}
}

func TestNewFromConfigUnknown(t *testing.T) {
	_, err := New(config.MemoryConfig{Provider: "wat"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
```

- [ ] **Step 2: Run the failing test**

Run: `go test ./tool/memory/memprovider/ -run TestNewFromConfig -v`
Expected: FAIL — `New` not defined.

- [ ] **Step 3: Write the factory**

Create `tool/memory/memprovider/factory.go`:

```go
package memprovider

import (
	"fmt"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
)

// New builds the active external memory provider from configuration.
// Returns (nil, nil) when no provider is configured. Returns an error
// when the provider name is unknown or the selected provider has no
// API key (so typos surface loudly instead of silently doing nothing).
func New(cfg config.MemoryConfig) (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if name == "" {
		return nil, nil
	}
	switch name {
	case "honcho":
		if cfg.Honcho.APIKey == "" {
			return nil, fmt.Errorf("memprovider: honcho requires api_key")
		}
		return NewHoncho(cfg.Honcho), nil
	case "mem0":
		if cfg.Mem0.APIKey == "" {
			return nil, fmt.Errorf("memprovider: mem0 requires api_key")
		}
		return NewMem0(cfg.Mem0), nil
	case "supermemory":
		if cfg.Supermemory.APIKey == "" {
			return nil, fmt.Errorf("memprovider: supermemory requires api_key")
		}
		return NewSupermemory(cfg.Supermemory), nil
	default:
		return nil, fmt.Errorf("memprovider: unknown provider %q", name)
	}
}
```

- [ ] **Step 4: Run the factory tests**

Run: `go test ./tool/memory/memprovider/...`
Expected: PASS for all memprovider tests.

- [ ] **Step 5: Wire up in cli/repl.go**

In `cli/repl.go`, add the import:

```go
"github.com/nousresearch/hermes-agent/tool/memory/memprovider"
```

Then, after the `memory.RegisterAll(toolRegistry, app.Storage)` block and before the `sessionID := uuid.NewString()` line, insert (replacing the existing sessionID assignment if needed):

```go
	sessionID := uuid.NewString()

	// External memory provider (Plan 6c): honcho / mem0 / supermemory.
	extMem, err := memprovider.New(app.Config.Memory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hermes: memory provider: %v\n", err)
	}
	if extMem != nil {
		if err := extMem.Initialize(ctx, sessionID); err != nil {
			fmt.Fprintf(os.Stderr, "hermes: memory provider %s init: %v\n", extMem.Name(), err)
		} else {
			extMem.RegisterTools(toolRegistry)
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = extMem.Shutdown(shutdownCtx)
			}()
		}
	}
```

Note: `sessionID := uuid.NewString()` already exists a few lines down. Move that assignment above the memprovider block (or reuse the existing one). The final file should have a single `sessionID` declaration.

- [ ] **Step 6: Run the full test suite**

Run: `go test ./...`
Expected: PASS — no existing tests should break, and the new memprovider tests all pass.

- [ ] **Step 7: Commit**

```bash
git add tool/memory/memprovider/factory.go tool/memory/memprovider/factory_test.go cli/repl.go
git commit -m "feat(cli+tool/memory): wire external memory provider into REPL bootstrap"
```

---

## Verification Checklist

- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `grep -r "memprovider" tool/memory/memprovider | wc -l` shows non-trivial usage
- [ ] REPL still starts when `memory.provider` is unset (`go run ./cmd/hermes repl` with a minimal config)
- [ ] Setting `memory.provider: wat` in config prints a warning and the REPL continues

## Out of scope (explicit)

- Hindsight local backend — deferred
- Byterover / Retaindb / Holographic / Openviking / Ex-Machina — deferred
- Background prefetch / async recall injection — deferred
- `SyncTurn` is fire-and-forget best-effort; failures are logged inside the handler but not surfaced to the user — acceptable for Plan 6c
