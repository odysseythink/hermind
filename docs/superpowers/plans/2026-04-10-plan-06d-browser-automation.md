# Plan 6d: Browser Automation (Browserbase) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give the agent a way to create and release Browserbase cloud browser sessions and hand back a live debug URL so the model can drive the browser (via downstream MCP / CDP tools, external integrations, or simply surface to the user).

**Architecture:** A thin Go package `tool/browser` wraps the Browserbase REST API (`POST /v1/sessions`, `POST /v1/sessions/{id}` with `REQUEST_RELEASE`, `GET /v1/sessions/{id}/debug` for live URLs). It exposes three tools: `browser_session_create`, `browser_session_close`, `browser_session_live_url`. Sessions are tracked in-memory inside a `SessionStore`. The CLI registers the toolset only when `BROWSERBASE_API_KEY` and `BROWSERBASE_PROJECT_ID` are present.

Full page-level automation (navigation, click, snapshot) requires CDP over WebSocket. That is intentionally out of scope — external MCP servers (`chrome-devtools-mcp`, `playwright-mcp`) cover it once the session URL is known.

**Tech Stack:** Go 1.25, stdlib `net/http`, existing `tool` registry, no new deps.

**Deliverable at end of plan:**
```
> open a browserbase session
⚡ browser_session_create: {}
│ {"session_id":"sess_...","connect_url":"wss://connect.browserbase.com/...","live_url":"https://www.browserbase.com/sessions/sess_..."}
└

> release it
⚡ browser_session_close: {"session_id":"sess_..."}
│ {"ok":true}
└
```

**Non-goals for this plan (deferred):**
- CDP WebSocket client / page interaction — deferred (use MCP)
- Browser Use backend — deferred
- Local Chromium backend — deferred
- Download fetching / artifact retrieval — deferred
- Firecrawl (already covered by `web_extract`)

**Plans 1-6c dependencies this plan touches:**
- `config/config.go` — add `BrowserConfig` block (optional, env vars work too)
- `tool/browser/` — NEW package (provider.go, browserbase.go, session_store.go, register.go)
- `cli/repl.go` — register browser tools when available

---

## File Structure

```
hermes-agent-go/
├── config/config.go                       # MODIFIED: add BrowserConfig
├── tool/
│   └── browser/                           # NEW
│       ├── provider.go                    # Provider interface
│       ├── session_store.go               # SessionStore (safe map)
│       ├── session_store_test.go
│       ├── browserbase.go                 # BrowserbaseProvider
│       ├── browserbase_test.go
│       ├── tools.go                       # Tool handlers
│       ├── register.go                    # RegisterAll
│       └── browser_test.go                # integration-lite
└── cli/repl.go                            # MODIFIED
```

---

## Task 1: Config block

- [ ] **Step 1:** Add to `config/config.go`:

```go
// BrowserConfig holds browser automation provider configuration.
// Only Browserbase is supported in Plan 6d.
type BrowserConfig struct {
	Provider    string `yaml:"provider,omitempty"` // "", "browserbase"
	Browserbase BrowserbaseConfig `yaml:"browserbase,omitempty"`
}

// BrowserbaseConfig holds Browserbase cloud provider settings.
// Env vars BROWSERBASE_API_KEY / BROWSERBASE_PROJECT_ID take precedence
// over the YAML values at load time (see tool/browser/browserbase.go).
type BrowserbaseConfig struct {
	BaseURL   string `yaml:"base_url,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`
	ProjectID string `yaml:"project_id,omitempty"`
	KeepAlive bool   `yaml:"keep_alive,omitempty"`
	Proxies   bool   `yaml:"proxies,omitempty"`
}
```

Add the field to `Config`:
```go
Browser           BrowserConfig             `yaml:"browser,omitempty"`
```

- [ ] **Step 2:** `go test ./config/...` — PASS.
- [ ] **Step 3:** Commit `feat(config): add BrowserConfig for Browserbase`.

---

## Task 2: Provider interface and session store

- [ ] **Step 1:** Create `tool/browser/provider.go`:

```go
// Package browser provides browser automation providers. Plan 6d ships
// only the Browserbase cloud backend. The tool surface is intentionally
// small: create, close, and fetch live debug URLs. Full page-level
// automation is expected to happen through MCP/CDP tooling once the
// session URL is known.
package browser

import "context"

// Session describes a created browser session.
type Session struct {
	ID         string `json:"id"`
	ConnectURL string `json:"connect_url"` // CDP WebSocket URL
	LiveURL    string `json:"live_url"`    // Debug / watch URL (may be empty)
	Provider   string `json:"provider"`
}

// Provider is the minimal browser backend interface. Implementations
// must be safe for concurrent use.
type Provider interface {
	Name() string
	IsConfigured() bool
	CreateSession(ctx context.Context) (*Session, error)
	CloseSession(ctx context.Context, id string) error
	LiveURL(ctx context.Context, id string) (string, error)
}
```

- [ ] **Step 2:** Create `tool/browser/session_store.go`:

```go
package browser

import "sync"

// SessionStore is a small thread-safe map of session IDs to Session
// records. Providers create sessions; the store gives the tool layer a
// place to remember which sessions it has handed out so we can look
// them up during close/live-url.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

func (s *SessionStore) Put(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

func (s *SessionStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *SessionStore) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Session, 0, len(s.sessions))
	for _, s := range s.sessions {
		out = append(out, s)
	}
	return out
}
```

- [ ] **Step 3:** Create `tool/browser/session_store_test.go`:

```go
package browser

import "testing"

func TestSessionStoreCRUD(t *testing.T) {
	s := NewSessionStore()
	s.Put(&Session{ID: "a", ConnectURL: "ws://a"})
	if got, ok := s.Get("a"); !ok || got.ConnectURL != "ws://a" {
		t.Fatalf("expected to get a, got %+v ok=%v", got, ok)
	}
	if len(s.List()) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.List()))
	}
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("expected delete to remove")
	}
}
```

- [ ] **Step 4:** Run `go test ./tool/browser/...` — PASS.
- [ ] **Step 5:** Commit `feat(tool/browser): add Provider interface and SessionStore`.

---

## Task 3: Browserbase provider

- [ ] **Step 1:** Create `tool/browser/browserbase.go`:

```go
package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nousresearch/hermes-agent/config"
)

// BrowserbaseProvider implements Provider against the Browserbase API.
type BrowserbaseProvider struct {
	cfg    config.BrowserbaseConfig
	client *http.Client
}

// NewBrowserbase builds a Browserbase provider from config, merging in
// environment variables if set.
func NewBrowserbase(cfg config.BrowserbaseConfig) *BrowserbaseProvider {
	if v := os.Getenv("BROWSERBASE_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("BROWSERBASE_PROJECT_ID"); v != "" {
		cfg.ProjectID = v
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.browserbase.com"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &BrowserbaseProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *BrowserbaseProvider) Name() string { return "browserbase" }

func (b *BrowserbaseProvider) IsConfigured() bool {
	return b.cfg.APIKey != "" && b.cfg.ProjectID != ""
}

// do sends a JSON body to the Browserbase API using the X-BB-API-Key
// auth header (Browserbase does not use Bearer auth).
func (b *BrowserbaseProvider) do(ctx context.Context, method, path string, body any, out any) error {
	if !b.IsConfigured() {
		return errors.New("browserbase: missing BROWSERBASE_API_KEY or BROWSERBASE_PROJECT_ID")
	}
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("browserbase: encode: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.cfg.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-BB-API-Key", b.cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("browserbase: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("browserbase: %s %s: status %d: %s",
			method, path, resp.StatusCode, string(bodyBytes))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type bbCreateResponse struct {
	ID         string `json:"id"`
	ConnectURL string `json:"connectUrl"`
}

// CreateSession calls POST /v1/sessions and returns the new session.
func (b *BrowserbaseProvider) CreateSession(ctx context.Context) (*Session, error) {
	body := map[string]any{"projectId": b.cfg.ProjectID}
	if b.cfg.KeepAlive {
		body["keepAlive"] = true
	}
	if b.cfg.Proxies {
		body["proxies"] = true
	}
	var resp bbCreateResponse
	if err := b.do(ctx, "POST", "/v1/sessions", body, &resp); err != nil {
		return nil, err
	}
	sess := &Session{
		ID:         resp.ID,
		ConnectURL: resp.ConnectURL,
		Provider:   b.Name(),
	}
	// Live URL is a separate endpoint; try once but tolerate failure.
	if live, err := b.LiveURL(ctx, resp.ID); err == nil {
		sess.LiveURL = live
	}
	return sess, nil
}

type bbDebugResponse struct {
	DebuggerFullscreenURL string `json:"debuggerFullscreenUrl"`
	DebuggerURL           string `json:"debuggerUrl"`
}

func (b *BrowserbaseProvider) LiveURL(ctx context.Context, id string) (string, error) {
	var resp bbDebugResponse
	if err := b.do(ctx, "GET", "/v1/sessions/"+id+"/debug", nil, &resp); err != nil {
		return "", err
	}
	if resp.DebuggerFullscreenURL != "" {
		return resp.DebuggerFullscreenURL, nil
	}
	return resp.DebuggerURL, nil
}

func (b *BrowserbaseProvider) CloseSession(ctx context.Context, id string) error {
	body := map[string]any{
		"projectId": b.cfg.ProjectID,
		"status":    "REQUEST_RELEASE",
	}
	return b.do(ctx, "POST", "/v1/sessions/"+id, body, nil)
}
```

- [ ] **Step 2:** Create `tool/browser/browserbase_test.go`:

```go
package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
)

func TestBrowserbaseCreateAndClose(t *testing.T) {
	var creates, releases, debugCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-BB-API-Key") != "k" {
			t.Errorf("missing api key header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/sessions" && r.Method == "POST":
			atomic.AddInt32(&creates, 1)
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["projectId"] != "proj" {
				t.Errorf("expected projectId=proj, got %v", body["projectId"])
			}
			_, _ = w.Write([]byte(`{"id":"sess_1","connectUrl":"wss://c/sess_1"}`))
		case strings.HasSuffix(r.URL.Path, "/debug") && r.Method == "GET":
			atomic.AddInt32(&debugCalls, 1)
			_, _ = w.Write([]byte(`{"debuggerFullscreenUrl":"https://live/sess_1"}`))
		case r.URL.Path == "/v1/sessions/sess_1" && r.Method == "POST":
			atomic.AddInt32(&releases, 1)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("BROWSERBASE_API_KEY", "k")
	t.Setenv("BROWSERBASE_PROJECT_ID", "proj")
	p := NewBrowserbase(config.BrowserbaseConfig{BaseURL: srv.URL})
	if !p.IsConfigured() {
		t.Fatal("expected configured")
	}

	ctx := context.Background()
	sess, err := p.CreateSession(ctx)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID != "sess_1" || sess.ConnectURL != "wss://c/sess_1" {
		t.Errorf("bad session: %+v", sess)
	}
	if sess.LiveURL != "https://live/sess_1" {
		t.Errorf("expected live url to be populated, got %q", sess.LiveURL)
	}
	if atomic.LoadInt32(&creates) != 1 {
		t.Errorf("creates = %d", creates)
	}
	if atomic.LoadInt32(&debugCalls) != 1 {
		t.Errorf("debugCalls = %d", debugCalls)
	}
	if err := p.CloseSession(ctx, "sess_1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if atomic.LoadInt32(&releases) != 1 {
		t.Errorf("releases = %d", releases)
	}
}

func TestBrowserbaseNotConfigured(t *testing.T) {
	t.Setenv("BROWSERBASE_API_KEY", "")
	t.Setenv("BROWSERBASE_PROJECT_ID", "")
	p := NewBrowserbase(config.BrowserbaseConfig{})
	if p.IsConfigured() {
		t.Fatal("expected not configured")
	}
	_, err := p.CreateSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3:** Run `go test ./tool/browser/...` — PASS.
- [ ] **Step 4:** Commit `feat(tool/browser): add Browserbase cloud provider`.

---

## Task 4: Tools + RegisterAll

- [ ] **Step 1:** Create `tool/browser/tools.go`:

```go
package browser

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nousresearch/hermes-agent/tool"
)

const createSchema = `{"type":"object","properties":{}}`

const closeSchema = `{
  "type":"object",
  "properties":{"session_id":{"type":"string"}},
  "required":["session_id"]
}`

const liveSchema = `{
  "type":"object",
  "properties":{"session_id":{"type":"string"}},
  "required":["session_id"]
}`

// newCreateHandler returns a handler that creates a new browser session
// via the given provider and stores the result in store.
func newCreateHandler(p Provider, store *SessionStore) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		sess, err := p.CreateSession(ctx)
		if err != nil {
			return tool.ToolError(err.Error()), nil
		}
		store.Put(sess)
		return tool.ToolResult(sess), nil
	}
}

// newCloseHandler returns a handler that releases a session.
func newCloseHandler(p Provider, store *SessionStore) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if strings.TrimSpace(args.SessionID) == "" {
			return tool.ToolError("session_id is required"), nil
		}
		if err := p.CloseSession(ctx, args.SessionID); err != nil {
			return tool.ToolError(err.Error()), nil
		}
		store.Delete(args.SessionID)
		return tool.ToolResult(map[string]any{"ok": true, "session_id": args.SessionID}), nil
	}
}

// newLiveURLHandler returns a handler that fetches the live debug URL.
func newLiveURLHandler(p Provider, store *SessionStore) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if strings.TrimSpace(args.SessionID) == "" {
			return tool.ToolError("session_id is required"), nil
		}
		url, err := p.LiveURL(ctx, args.SessionID)
		if err != nil {
			return tool.ToolError(err.Error()), nil
		}
		if sess, ok := store.Get(args.SessionID); ok {
			sess.LiveURL = url
		}
		return tool.ToolResult(map[string]any{"session_id": args.SessionID, "live_url": url}), nil
	}
}
```

- [ ] **Step 2:** Create `tool/browser/register.go`:

```go
package browser

import (
	"encoding/json"

	"github.com/nousresearch/hermes-agent/tool"
)

// RegisterAll registers the browser toolset against reg if the provider
// is configured. Callers pass an already-constructed Provider; if it
// returns false from IsConfigured(), no tools are registered.
func RegisterAll(reg *tool.Registry, p Provider) {
	if p == nil || !p.IsConfigured() {
		return
	}
	store := NewSessionStore()

	reg.Register(&tool.Entry{
		Name:        "browser_session_create",
		Toolset:     "browser",
		Description: "Create a new cloud browser session and return its connect URL.",
		Emoji:       "🌐",
		Handler:     newCreateHandler(p, store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "browser_session_create",
				Description: "Create a new Browserbase cloud browser session. Returns the session ID, CDP connect URL, and live debug URL.",
				Parameters:  json.RawMessage(createSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "browser_session_close",
		Toolset:     "browser",
		Description: "Release a browser session.",
		Emoji:       "🧹",
		Handler:     newCloseHandler(p, store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "browser_session_close",
				Description: "Release a previously created browser session.",
				Parameters:  json.RawMessage(closeSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "browser_session_live_url",
		Toolset:     "browser",
		Description: "Fetch the live debugger URL for a browser session.",
		Emoji:       "🔭",
		Handler:     newLiveURLHandler(p, store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "browser_session_live_url",
				Description: "Return the live debugger URL for a browser session so the user or a downstream tool can watch it.",
				Parameters:  json.RawMessage(liveSchema),
			},
		},
	})
}
```

- [ ] **Step 3:** Create `tool/browser/browser_test.go`:

```go
package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

func TestRegisterAllAndDispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/sessions" && r.Method == "POST":
			_, _ = w.Write([]byte(`{"id":"sess_42","connectUrl":"wss://c/sess_42"}`))
		case strings.HasSuffix(r.URL.Path, "/debug") && r.Method == "GET":
			_, _ = w.Write([]byte(`{"debuggerFullscreenUrl":"https://live/sess_42"}`))
		case r.URL.Path == "/v1/sessions/sess_42" && r.Method == "POST":
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("BROWSERBASE_API_KEY", "k")
	t.Setenv("BROWSERBASE_PROJECT_ID", "proj")
	p := NewBrowserbase(config.BrowserbaseConfig{BaseURL: srv.URL})

	reg := tool.NewRegistry()
	RegisterAll(reg, p)

	ctx := context.Background()
	res, err := reg.Dispatch(ctx, "browser_session_create", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("create dispatch: %v", err)
	}
	if !strings.Contains(res, "sess_42") {
		t.Errorf("missing session id in create result: %s", res)
	}

	closeArgs, _ := json.Marshal(map[string]string{"session_id": "sess_42"})
	res, err = reg.Dispatch(ctx, "browser_session_close", closeArgs)
	if err != nil {
		t.Fatalf("close dispatch: %v", err)
	}
	if !strings.Contains(res, `"ok":true`) {
		t.Errorf("expected ok:true, got %s", res)
	}
}

func TestRegisterAllSkipsUnconfigured(t *testing.T) {
	t.Setenv("BROWSERBASE_API_KEY", "")
	t.Setenv("BROWSERBASE_PROJECT_ID", "")
	reg := tool.NewRegistry()
	p := NewBrowserbase(config.BrowserbaseConfig{})
	RegisterAll(reg, p)
	if len(reg.Definitions(nil)) != 0 {
		t.Errorf("expected no tools registered when not configured")
	}
}
```

- [ ] **Step 4:** `go test ./tool/browser/...` — PASS.
- [ ] **Step 5:** Commit `feat(tool/browser): add session tools and RegisterAll`.

---

## Task 5: CLI wiring

- [ ] **Step 1:** In `cli/repl.go`, after the MCP wiring (or near the web tools block), add:

```go
// Browser automation tools (Plan 6d). Registered only when Browserbase
// credentials are present.
browserProvider := browser.NewBrowserbase(app.Config.Browser.Browserbase)
browser.RegisterAll(toolRegistry, browserProvider)
```

And add the import `"github.com/nousresearch/hermes-agent/tool/browser"`.

- [ ] **Step 2:** `go build ./...` — PASS.
- [ ] **Step 3:** `go test ./...` — PASS.
- [ ] **Step 4:** Commit `feat(cli): register Browserbase tools when configured`.

---

## Verification Checklist

- [ ] `go build ./...` / `go test ./...` pass
- [ ] Without `BROWSERBASE_API_KEY`, `hermes repl` registers 0 browser tools
- [ ] With both env vars set, the tools appear in `Definitions`
