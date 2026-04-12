# Phase 5: Camofox Browser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a local Camofox anti-detection browser provider alongside the existing Browserbase cloud provider.

**Architecture:** Implement the existing `browser.Provider` interface (Name/IsConfigured/CreateSession/CloseSession/LiveURL) as an HTTP REST client to a self-hosted Camofox server. Add `CamofoxConfig` to the config package. Zero new Go dependencies — pure HTTP calls.

**Tech Stack:** Go 1.25, stdlib `net/http`, `encoding/json`

---

## File Structure

```
hermes-agent-go/
├── config/
│   └── config.go           # Add CamofoxConfig struct + Camofox field to BrowserConfig
├── tool/browser/
│   ├── provider.go          # (existing, unchanged)
│   ├── browserbase.go       # (existing, unchanged)
│   ├── camofox.go           # Camofox provider implementation
│   └── camofox_test.go      # Tests with mock HTTP server
```

---

### Task 1: Add CamofoxConfig to config package

**Files:**
- Modify: `hermes-agent-go/config/config.go`

- [ ] **Step 1: Add CamofoxConfig struct after BrowserbaseConfig**

In `hermes-agent-go/config/config.go`, after the `BrowserbaseConfig` struct (line 82), add:

```go
// CamofoxConfig holds Camofox local browser provider settings.
type CamofoxConfig struct {
	BaseURL            string `yaml:"base_url,omitempty"`            // default http://localhost:9377
	ManagedPersistence bool   `yaml:"managed_persistence,omitempty"` // reuse profiles per user ID
}
```

- [ ] **Step 2: Add Camofox field to BrowserConfig**

In the `BrowserConfig` struct, add the Camofox field:

```go
type BrowserConfig struct {
	Provider    string            `yaml:"provider,omitempty"` // "", "browserbase", "camofox"
	Browserbase BrowserbaseConfig `yaml:"browserbase,omitempty"`
	Camofox     CamofoxConfig     `yaml:"camofox,omitempty"`
}
```

- [ ] **Step 3: Verify it compiles**

```bash
cd hermes-agent-go && go build ./config/
```

Expected: Compiles.

- [ ] **Step 4: Run existing tests**

```bash
cd hermes-agent-go && go test ./...
```

Expected: All pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/config/config.go
git commit -m "feat(config): add CamofoxConfig for local browser provider"
```

---

### Task 2: Implement Camofox provider

**Files:**
- Create: `hermes-agent-go/tool/browser/camofox.go`
- Create: `hermes-agent-go/tool/browser/camofox_test.go`

- [ ] **Step 1: Write the failing test**

Create `hermes-agent-go/tool/browser/camofox_test.go`:

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

func TestCamofoxCreateAndClose(t *testing.T) {
	var creates, closes int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/sessions" && r.Method == "POST":
			atomic.AddInt32(&creates, 1)
			_, _ = w.Write([]byte(`{
				"id": "sess_1",
				"cdp_url": "ws://localhost:9222/devtools/browser/abc",
				"vnc_url": "http://localhost:5900"
			}`))
		case strings.HasPrefix(r.URL.Path, "/sessions/sess_1") && r.Method == "DELETE":
			atomic.AddInt32(&closes, 1)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewCamofox(config.CamofoxConfig{BaseURL: srv.URL})
	if !p.IsConfigured() {
		t.Fatal("expected configured")
	}
	if p.Name() != "camofox" {
		t.Errorf("Name() = %q", p.Name())
	}

	ctx := context.Background()
	sess, err := p.CreateSession(ctx)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID != "sess_1" {
		t.Errorf("id = %q", sess.ID)
	}
	if sess.ConnectURL != "ws://localhost:9222/devtools/browser/abc" {
		t.Errorf("connect_url = %q", sess.ConnectURL)
	}
	if sess.LiveURL != "http://localhost:5900" {
		t.Errorf("live_url = %q", sess.LiveURL)
	}
	if sess.Provider != "camofox" {
		t.Errorf("provider = %q", sess.Provider)
	}
	if atomic.LoadInt32(&creates) != 1 {
		t.Errorf("creates = %d", creates)
	}

	if err := p.CloseSession(ctx, "sess_1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if atomic.LoadInt32(&closes) != 1 {
		t.Errorf("closes = %d", closes)
	}
}

func TestCamofoxNotConfigured(t *testing.T) {
	p := NewCamofox(config.CamofoxConfig{})
	if p.IsConfigured() {
		t.Fatal("expected not configured with empty config")
	}
}

func TestCamofoxLiveURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/sessions/sess_1" && r.Method == "GET" {
			_, _ = w.Write([]byte(`{"vnc_url": "http://localhost:5900/sess_1"}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewCamofox(config.CamofoxConfig{BaseURL: srv.URL})
	url, err := p.LiveURL(context.Background(), "sess_1")
	if err != nil {
		t.Fatalf("LiveURL: %v", err)
	}
	if url != "http://localhost:5900/sess_1" {
		t.Errorf("url = %q", url)
	}
}

func TestCamofoxManagedPersistence(t *testing.T) {
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/sessions" && r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&lastBody)
			_, _ = w.Write([]byte(`{"id":"sess_1","cdp_url":"ws://x","vnc_url":""}`))
		}
	}))
	defer srv.Close()

	p := NewCamofox(config.CamofoxConfig{
		BaseURL:            srv.URL,
		ManagedPersistence: true,
	})
	_, _ = p.CreateSession(ctx)
	// When ManagedPersistence is true, the request body should include persist: true.
	if lastBody["persist"] != true {
		t.Errorf("expected persist=true, got %v", lastBody["persist"])
	}
}
```

Note: The `TestCamofoxManagedPersistence` test uses a package-level `ctx` — replace with `context.Background()`.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./tool/browser/ -run TestCamofox -v
```

Expected: Compilation error — `NewCamofox` undefined.

- [ ] **Step 3: Implement camofox.go**

Create `hermes-agent-go/tool/browser/camofox.go`:

```go
package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nousresearch/hermes-agent/config"
)

// CamofoxProvider implements Provider against a self-hosted Camofox
// anti-detection browser server.
type CamofoxProvider struct {
	cfg    config.CamofoxConfig
	client *http.Client
}

// NewCamofox builds a Camofox provider from config.
func NewCamofox(cfg config.CamofoxConfig) *CamofoxProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:9377"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &CamofoxProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CamofoxProvider) Name() string { return "camofox" }

func (c *CamofoxProvider) IsConfigured() bool {
	return c.cfg.BaseURL != ""
}

type camofoxCreateResponse struct {
	ID     string `json:"id"`
	CDPURL string `json:"cdp_url"`
	VNCURL string `json:"vnc_url"`
}

func (c *CamofoxProvider) CreateSession(ctx context.Context) (*Session, error) {
	body := map[string]any{}
	if c.cfg.ManagedPersistence {
		body["persist"] = true
	}

	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/sessions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("camofox: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("camofox: create session: status %d: %s", resp.StatusCode, string(respBody))
	}

	var cr camofoxCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("camofox: decode: %w", err)
	}

	return &Session{
		ID:         cr.ID,
		ConnectURL: cr.CDPURL,
		LiveURL:    cr.VNCURL,
		Provider:   c.Name(),
	}, nil
}

func (c *CamofoxProvider) CloseSession(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.cfg.BaseURL+"/sessions/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("camofox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("camofox: close session: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

type camofoxSessionResponse struct {
	VNCURL string `json:"vnc_url"`
}

func (c *CamofoxProvider) LiveURL(ctx context.Context, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/sessions/"+id, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("camofox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("camofox: get session: status %d: %s", resp.StatusCode, string(body))
	}
	var sr camofoxSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("camofox: decode: %w", err)
	}
	return sr.VNCURL, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd hermes-agent-go && go test ./tool/browser/ -run TestCamofox -v
```

Expected: All 4 Camofox tests pass.

- [ ] **Step 5: Run all browser tests**

```bash
cd hermes-agent-go && go test ./tool/browser/...
```

Expected: All pass (including existing Browserbase tests).

- [ ] **Step 6: Verify Provider interface compliance**

Add to the test file (or verify the implementation compiles with):

```go
var _ Provider = (*CamofoxProvider)(nil)
```

- [ ] **Step 7: Run full project tests**

```bash
cd hermes-agent-go && go test ./...
```

Expected: All pass.

- [ ] **Step 8: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/tool/browser/camofox.go hermes-agent-go/tool/browser/camofox_test.go
git commit -m "feat(browser): add Camofox local anti-detection browser provider"
```
