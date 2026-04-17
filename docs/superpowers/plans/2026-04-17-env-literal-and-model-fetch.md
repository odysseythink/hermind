# Literal API Keys + Per-Provider Model Fetch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Drop `env:VAR` indirection from the config loader (breaking change) and add a per-provider "Get" button in the web UI that queries the provider's `/models` endpoint via a new optional `provider.ModelLister` interface, populating a `<datalist>` on the provider card's `model` field.

**Architecture:** Two pieces land together. (1) `config/loader.go` loses `expandEnvVars`; `cli/setup.go` stops emitting `env:VAR` in generated configs (env is read once at setup time, baked into the YAML as a literal). (2) New `provider.ModelLister` optional interface, implemented once on `*openaicompat.Client` (covers DeepSeek, Kimi, Qwen, Minimax, OpenAI, OpenAICompat, OpenRouter — six delegates plus openaicompat itself) and once on `*anthropic.Anthropic`. Webconfig adds `POST /api/providers/models` that type-asserts the instantiated provider and returns the model list. Frontend grows a trailing `Get` button next to each provider card's `model` input, backed by an empty `<datalist>` that fills on click; live-reactive to the `api_key` input.

**Tech Stack:** Go (`net/http`, `httptest`, existing `provider/factory` and `config/editor` packages), plain JS/CSS served via `//go:embed`, no new dependencies.

---

## File Structure

- Modify: `config/loader.go` — delete `expandEnvVars` and its caller inside `LoadFromPath`.
- Modify: `config/loader_test.go` — delete `TestEnvVarExpansion` + `TestEnvVarExpansionRejectsEmpty`; update `TestLoadFromYAMLParsesMCPServers` to use a literal token; add `TestLoadPreservesLiteralEnvString`.
- Modify: `cli/setup.go` — when generating a new config, resolve `os.Getenv(EnvVar)` at setup time and write the literal; update the header comment and the commented-out disabled-provider templates accordingly.
- Modify: `provider/provider.go` — add `ModelLister` optional interface.
- Create: `provider/openaicompat/list_models.go` — `(*Client).ListModels(ctx)`.
- Create: `provider/openaicompat/list_models_test.go` — happy + failure cases.
- Create: `provider/anthropic/list_models.go` — `(*Anthropic).ListModels(ctx)`.
- Create: `provider/anthropic/list_models_test.go` — happy + failure + header assertions.
- Modify: `cli/ui/webconfig/handlers.go` — add `handleProvidersModels` + `providersModelsUnsupported` logic.
- Modify: `cli/ui/webconfig/server.go` — mount `/api/providers/models`.
- Modify: `cli/ui/webconfig/server_test.go` — three new tests.
- Modify: `cli/ui/webconfig/web/app.js` — Get button wiring, datalist, live api_key reactivity, flush-on-click.
- Modify: `cli/ui/webconfig/web/app.css` — `.get-models-btn`, `.inline-error`, `.model-row` flex layout.

**Untouched:** TUI editor (`cli/ui/config/`), provider packages that delegate to `openaicompat` (they inherit `ListModels` via embedding), `zhipu` and `wenxin` packages (intentionally don't implement `ModelLister`).

**Commit shape:** Spec rollout calls for two logical commits (`refactor(config): drop env:VAR ...` then `feat(web-config): per-provider Get models button`). Task 1 closes with the refactor commit; Tasks 2–7 accumulate in the working tree and the feat commit lands at the end of Task 8. Intermediate verification happens in each task; no intermediate commits.

---

## Task 1: Remove `env:VAR` expansion from config loader + `hermind setup`

**Files:**
- Modify: `config/loader.go`
- Modify: `config/loader_test.go`
- Modify: `cli/setup.go`

- [ ] **Step 1: Delete the `expandEnvVars` call in `LoadFromPath`**

In `config/loader.go`, remove lines 48-50:

```go
	if err := expandEnvVars(cfg); err != nil {
		return nil, err
	}
```

`LoadFromPath` after the edit:

```go
func LoadFromPath(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		resolveDefaults(cfg)
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	resolveDefaults(cfg)
	return cfg, nil
}
```

- [ ] **Step 2: Delete the `expandEnvVars` function from `config/loader.go`**

Remove the entire function body, lines 71-128 inclusive (the block starting with the `// expandEnvVars` comment and ending with the closing `}` of the function).

- [ ] **Step 3: Remove the `os` and `strings` imports if unused**

After deletion, re-check the import block at the top of `loader.go`. `os` is still used by `resolveDefaults` and `LoadFromPath`, `strings` is still used by `expandPath`. Leave imports as-is.

- [ ] **Step 4: Delete `TestEnvVarExpansion` and `TestEnvVarExpansionRejectsEmpty`**

In `config/loader_test.go`, delete both tests entirely (lines 53-84 in the current file — the two `TestEnvVarExpansion*` functions).

- [ ] **Step 5: Update `TestLoadFromYAMLParsesMCPServers` to use a literal token**

Replace the existing test (lines 123-152) with:

```go
func TestLoadFromYAMLParsesMCPServers(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
mcp:
  servers:
    github:
      command: npx
      args: [-y, "@modelcontextprotocol/server-github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: gh-secret-literal
    filesystem:
      command: npx
      args: [-y, "@modelcontextprotocol/server-filesystem", "/tmp"]
      enabled: false
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	require.Contains(t, cfg.MCP.Servers, "github")
	assert.Equal(t, "npx", cfg.MCP.Servers["github"].Command)
	assert.Equal(t, "gh-secret-literal", cfg.MCP.Servers["github"].Env["GITHUB_PERSONAL_ACCESS_TOKEN"])
	assert.True(t, cfg.MCP.Servers["github"].IsEnabled())

	require.Contains(t, cfg.MCP.Servers, "filesystem")
	assert.False(t, cfg.MCP.Servers["filesystem"].IsEnabled())
}
```

Key changes: the `t.Setenv("GITHUB_TOKEN", ...)` line is gone, and the YAML uses a literal `gh-secret-literal` instead of `env:GITHUB_TOKEN`.

- [ ] **Step 6: Add `TestLoadPreservesLiteralEnvString`**

Append to `config/loader_test.go` (after `TestLoadFromYAMLParsesFallbackProviders`):

```go
func TestLoadPreservesLiteralEnvString(t *testing.T) {
	// After dropping env:VAR expansion, a config value that happens to start
	// with "env:" must round-trip as a literal string, not trigger lookup.
	t.Setenv("HERMIND_TEST_KEY", "should-not-be-used")
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(yamlPath, []byte(`
providers:
  anthropic:
    provider: anthropic
    api_key: env:HERMIND_TEST_KEY
`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadFromPath(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "env:HERMIND_TEST_KEY", cfg.Providers["anthropic"].APIKey)
}
```

- [ ] **Step 7: Update `cli/setup.go` so `hermind setup` stops writing `env:VAR`**

In `cli/setup.go`, two code blocks and two comments need to change.

First, the header comment around line 87. Replace:

```go
	b.WriteString("# API keys may be inlined or referenced via env: prefix, e.g. env:ANTHROPIC_API_KEY\n\n")
```

with:

```go
	b.WriteString("# Paste API keys as literal strings, e.g. api_key: \"sk-ant-...\"\n\n")
```

Second, the active-provider template around line 144-148. Replace:

```go
	if apiKey == "" && p.EnvVar != "" {
		fmt.Fprintf(&b, "    api_key: env:%s\n", p.EnvVar)
	} else {
		fmt.Fprintf(&b, "    api_key: %q\n", apiKey)
	}
```

with:

```go
	if apiKey == "" && p.EnvVar != "" {
		// Resolve env var once at setup time; no env: indirection at load.
		apiKey = os.Getenv(p.EnvVar)
	}
	fmt.Fprintf(&b, "    api_key: %q\n", apiKey)
```

Third, the disabled-provider commented-out template around line 163. Replace:

```go
	fmt.Fprintf(&b, "  #   api_key: env:%s\n", p.EnvVar)
```

with:

```go
	fmt.Fprintf(&b, "  #   api_key: \"\"\n")
```

Fourth, the wenxin-specific comments at lines 151 and 166. Replace:

```go
	if p.Name == "wenxin" {
		b.WriteString("    # wenxin also requires env:WENXIN_SECRET_KEY\n")
	}
```

with:

```go
	if p.Name == "wenxin" {
		b.WriteString("    # wenxin also requires secret_key (paste as literal string)\n")
	}
```

And for the disabled-provider version (line 166):

```go
	if p.Name == "wenxin" {
		b.WriteString("  #   # wenxin also requires secret_key (paste as literal string)\n")
	}
```

- [ ] **Step 8: `os` is already imported in `cli/setup.go`**

The file already imports `os` at line 6 (used by `os.Stdin`). No import change needed for the new `os.Getenv` call.

- [ ] **Step 9: Run the config tests**

Run: `go test ./config/...`
Expected: PASS. No failures, no new test regressions.

- [ ] **Step 10: Run the CLI tests**

Run: `go test ./cli/...`
Expected: PASS. (There is no dedicated setup_test.go; we're verifying the package still compiles and other CLI tests aren't affected.)

- [ ] **Step 11: Run the full test suite**

Run: `go test ./...`
Expected: PASS. No unexpected failures from removing env expansion.

- [ ] **Step 12: Commit the refactor**

```bash
git add config/loader.go config/loader_test.go cli/setup.go
git commit -m "$(cat <<'EOF'
refactor(config): drop env:VAR api-key indirection

Deletes expandEnvVars and stops resolving env:VAR prefixes in
providers[].api_key, fallback_providers[].api_key,
terminal.{modal,daytona}_token, and mcp.servers[].env values.
Literal strings round-trip as-is.

hermind setup now resolves the conventional env var once at setup
time and bakes the literal value into the generated YAML. No runtime
indirection remains.

BREAKING: Existing configs containing "api_key: env:FOO" will now
send the literal "env:FOO" string to providers and 401. Paste the
real key via hermind config --web.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add the `ModelLister` optional interface

**Files:**
- Modify: `provider/provider.go`

- [ ] **Step 1: Append the interface definition to `provider/provider.go`**

Add at the end of the file (after the existing `Provider` interface and other types):

```go
// ModelLister is an optional capability for providers that expose a
// models-listing endpoint. Webconfig consumers do a type assertion
// (`lister, ok := p.(ModelLister)`) before offering model discovery.
// Providers whose backend does not expose model listing simply do not
// implement this interface; callers should handle the negative assert.
type ModelLister interface {
	// ListModels returns the model IDs advertised by the provider.
	// Ordering is provider-defined (typically the server's response
	// order preserved). An empty slice is a valid result. Errors
	// carry the underlying HTTP status or transport error; callers
	// should surface them without further wrapping.
	ListModels(ctx context.Context) ([]string, error)
}
```

`context` is already imported by `provider.go` (used in `Provider.Complete`). No import change needed.

- [ ] **Step 2: Verify the package compiles**

Run: `go build ./provider/...`
Expected: no errors.

- [ ] **Step 3: Do NOT commit — this is part of the feat rollup**

This interface has no behavior on its own. It ships together with its implementations in the final feat commit (Task 8). Leave the change in the working tree.

---

## Task 3: Implement `(*openaicompat.Client).ListModels`

**Files:**
- Create: `provider/openaicompat/list_models.go`
- Create: `provider/openaicompat/list_models_test.go`

- [ ] **Step 1: Write the failing happy-path test first**

Create `provider/openaicompat/list_models_test.go`:

```go
package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListModelsHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/models", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "model-a"},
				{"id": "model-b"},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "model-a",
	})
	require.NoError(t, err)

	got, err := c.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"model-a", "model-b"}, got)
}

func TestListModelsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "model-a",
	})
	require.NoError(t, err)

	_, err = c.ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestListModelsIncludesExtraHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "hermind-test", r.Header.Get("X-Foo"))
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{}})
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		BaseURL:      srv.URL,
		APIKey:       "test-key",
		Model:        "model-a",
		ExtraHeaders: map[string]string{"X-Foo": "hermind-test"},
	})
	require.NoError(t, err)

	_, err = c.ListModels(context.Background())
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run the test, expect a compile failure (method not defined)**

Run: `go test ./provider/openaicompat/ -run ListModels`
Expected: build fails with `c.ListModels undefined` (or similar).

- [ ] **Step 3: Implement `ListModels` on `*Client`**

Create `provider/openaicompat/list_models.go`:

```go
package openaicompat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ListModels queries GET {BaseURL}/models with the configured Bearer
// auth and parses the OpenAI-standard `{"data":[{"id":...}]}` shape.
// Returns model IDs in the order the server returned them. Satisfies
// provider.ModelLister.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("openaicompat list models: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	for k, v := range c.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openaicompat list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("openaicompat list models: %d: %s", resp.StatusCode, string(body))
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("openaicompat list models: decode: %w", err)
	}

	ids := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}
```

The HTTP client field on `Client` is named `http` (not `httpClient`). Verified in `provider/openaicompat/openaicompat.go:46`.

- [ ] **Step 4: Run the tests, expect PASS**

Run: `go test ./provider/openaicompat/ -run ListModels -v`
Expected: three PASS lines.

- [ ] **Step 5: Run the full openaicompat test suite (regression check)**

Run: `go test ./provider/openaicompat/`
Expected: PASS. No pre-existing test breaks.

- [ ] **Step 6: Do NOT commit — part of the feat rollup**

---

## Task 4: Implement `(*anthropic.Anthropic).ListModels`

**Files:**
- Create: `provider/anthropic/list_models.go`
- Create: `provider/anthropic/list_models_test.go`

- [ ] **Step 1: Write the failing tests**

Create `provider/anthropic/list_models_test.go`:

```go
package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicListModelsHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, defaultAPIVersion, r.Header.Get("anthropic-version"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "claude-opus-4-6"},
				{"id": "claude-sonnet-4-6"},
			},
		})
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "anthropic",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	a := p.(*Anthropic)
	got, err := a.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"claude-opus-4-6", "claude-sonnet-4-6"}, got)
}

func TestAnthropicListModelsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	p, err := New(config.ProviderConfig{
		Provider: "anthropic",
		BaseURL:  srv.URL,
		APIKey:   "bad-key",
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	a := p.(*Anthropic)
	_, err = a.ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
```

- [ ] **Step 2: Run the tests, expect compile failure**

Run: `go test ./provider/anthropic/ -run ListModels`
Expected: fails with `a.ListModels undefined`.

- [ ] **Step 3: Implement `ListModels` on `*Anthropic`**

Create `provider/anthropic/list_models.go`:

```go
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ListModels queries GET {BaseURL}/v1/models with Anthropic's header
// shape (x-api-key, anthropic-version). Parses the OpenAI-compatible
// `{"data":[{"id":...}]}` response. Satisfies provider.ModelLister.
func (a *Anthropic) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("anthropic list models: %w", err)
	}
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", defaultAPIVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("anthropic list models: %d: %s", resp.StatusCode, string(body))
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("anthropic list models: decode: %w", err)
	}

	ids := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}
```

The HTTP client field on `Anthropic` is named `client`. Verified in `provider/anthropic/anthropic.go:24`.

- [ ] **Step 4: Run the tests, expect PASS**

Run: `go test ./provider/anthropic/ -run ListModels -v`
Expected: two PASS lines.

- [ ] **Step 5: Run the full anthropic test suite**

Run: `go test ./provider/anthropic/`
Expected: PASS.

- [ ] **Step 6: Do NOT commit — part of the feat rollup**

---

## Task 5: Add `POST /api/providers/models` webconfig endpoint

**Files:**
- Modify: `cli/ui/webconfig/handlers.go`
- Modify: `cli/ui/webconfig/server.go`
- Modify: `cli/ui/webconfig/server_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `cli/ui/webconfig/server_test.go` (just before the `TestServeRespectsContextCancel` block):

```go
func TestProvidersModelsHappyPath(t *testing.T) {
	// Canned provider /models endpoint.
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "aa"},
				{"id": "bb"},
			},
		})
	}))
	defer providerSrv.Close()

	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	yamlBody := []byte("providers:\n  test:\n    provider: openai\n    base_url: " + providerSrv.URL + "\n    api_key: sk-test\n    model: aa\n")
	if err := os.WriteFile(p, yamlBody, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"key": "test"})
	resp, err := http.Post(ts.URL+"/api/providers/models", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, readBody(resp))
	}
	var out struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Models) != 2 || out.Models[0] != "aa" || out.Models[1] != "bb" {
		t.Errorf("unexpected models: %+v", out.Models)
	}
}

func TestProvidersModelsUnsupportedType(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	// zhipu is in the unsupported set — no ListModels implementation.
	os.WriteFile(p, []byte("providers:\n  test:\n    provider: zhipu\n    api_key: sk-test\n"), 0o644)
	s, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"key": "test"})
	resp, err := http.Post(ts.URL+"/api/providers/models", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestProvidersModelsOriginCheck(t *testing.T) {
	ts, _ := newServer(t)
	body, _ := json.Marshal(map[string]string{"key": "test"})
	req, _ := http.NewRequest("POST", ts.URL+"/api/providers/models", bytes.NewReader(body))
	req.Header.Set("Origin", "https://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
```

Note: `io` is already imported in the existing test file (used by `readBody` and the other tests). Verify by reading the import block; if `io` is missing, add it.

- [ ] **Step 2: Run the tests, expect failure**

Run: `go test ./cli/ui/webconfig/ -run "TestProvidersModels"`
Expected: all three fail (404 for the endpoint — it doesn't exist yet).

- [ ] **Step 3: Add the handler to `cli/ui/webconfig/handlers.go`**

Append to `handlers.go` (after `handleProviders`, before `handleShutdown`):

```go
// handleProvidersModels queries the provider's /models endpoint using
// credentials from the in-memory doc and returns the list of model IDs.
// Requires loopback origin (same defense as /api/reveal) since it uses
// the user's live API key against third-party services.
func (s *Server) handleProvidersModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isLocalOrigin(r) {
		http.Error(w, "cross-origin denied", http.StatusForbidden)
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !validProviderKey(body.Key) {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}

	providerType, _ := s.doc.Get("providers." + body.Key + ".provider")
	baseURL, _ := s.doc.Get("providers." + body.Key + ".base_url")
	apiKey, _ := s.doc.Get("providers." + body.Key + ".api_key")
	model, _ := s.doc.Get("providers." + body.Key + ".model")

	cfg := config.ProviderConfig{
		Provider: providerType,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Model:    model,
	}
	p, err := factory.New(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lister, ok := p.(provider.ModelLister)
	if !ok {
		http.Error(w, fmt.Sprintf("provider type %q does not support model listing", providerType), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	models, err := lister.ListModels(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"models": models})
}
```

- [ ] **Step 4: Add the imports to `handlers.go`**

Add to the import block at the top of `cli/ui/webconfig/handlers.go`:

```go
	"fmt"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
```

Keep existing imports (`context`, `encoding/json`, `net/http`, `net/url`, `strings`, `time`, `github.com/odysseythink/hermind/config/editor`).

- [ ] **Step 5: Register the route in `cli/ui/webconfig/server.go`**

In `Handler()`, add the route right after `/api/providers`:

```go
	mux.HandleFunc("/api/providers/models", s.handleProvidersModels)
```

The full handler registration block should now read:

```go
	mux.HandleFunc("/api/schema", s.handleSchema)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/providers", s.handleProviders)
	mux.HandleFunc("/api/providers/models", s.handleProvidersModels)
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/reveal", s.handleReveal)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)
```

Note: `net/http.ServeMux` treats `/api/providers/models` as more specific than `/api/providers`, so there's no routing clash.

- [ ] **Step 6: Run the new tests**

Run: `go test ./cli/ui/webconfig/ -run "TestProvidersModels" -v`
Expected: all three PASS.

- [ ] **Step 7: Run the full webconfig test suite**

Run: `go test ./cli/ui/webconfig/...`
Expected: PASS. No regressions in existing tests (`TestProvidersGETMasksAPIKey`, `TestProvidersAddSetDelete`, `TestProvidersRejectsInvalidKey`, `TestServeRespectsContextCancel`, etc.).

- [ ] **Step 8: Do NOT commit — part of the feat rollup**

---

## Task 6: Frontend — Get button, datalist, live api_key reactivity

**Files:**
- Modify: `cli/ui/webconfig/web/app.js`
- Modify: `cli/ui/webconfig/web/app.css`

- [ ] **Step 1: Add CSS for the Get button, model row, and inline error**

Append to `cli/ui/webconfig/web/app.css` (after the `.provider-add` rule, before `/* List placeholder */`):

```css
.model-row {
  position: relative;
  display: flex;
  align-items: stretch;
  gap: 8px;
}
.model-row input {
  flex: 1;
}
.get-models-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0 12px;
  font: inherit;
  font-size: 13px;
  color: var(--muted);
  cursor: default;
  transition: color 120ms ease, border-color 120ms ease;
}
.get-models-btn.active {
  color: var(--accent);
  cursor: pointer;
}
.get-models-btn.active:hover {
  border-color: var(--accent);
}
.get-models-btn:disabled {
  opacity: 0.7;
}
.inline-error {
  display: block;
  margin-top: 4px;
  font-size: 13px;
  color: var(--error);
}
```

- [ ] **Step 2: Update `providerRow` in `app.js` to emit the new structure for the `model` field**

In `cli/ui/webconfig/web/app.js`, find the existing `providerRow(label, p, field, kind)` function. Replace its entire body with:

```js
function providerRow(label, p, field, kind) {
  const wrap = document.createElement('label');
  const lbl = document.createElement('span'); lbl.className = 'lbl'; lbl.textContent = label;
  wrap.appendChild(lbl);
  if (kind === 'secret') {
    const box = document.createElement('span');
    box.className = 'secret-wrap';
    const inp = document.createElement('input');
    inp.type = 'password';
    inp.value = p[field] || '';
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'reveal-btn';
    btn.textContent = 'Show';
    btn.onclick = async () => {
      if (inp.type === 'password') {
        const r = await fetch('/api/reveal', {
          method: 'POST',
          body: JSON.stringify({path: 'providers.' + p.key + '.api_key'}),
        });
        if (r.ok) {
          const b = await r.json();
          inp.value = b.value;
          inp.type = 'text';
          btn.textContent = 'Hide';
        }
      } else {
        inp.type = 'password';
        btn.textContent = 'Show';
      }
    };
    inp.oninput = () => updateGetBtn(p.key, inp.value);
    inp.onchange = () => persistProviderField(p.key, field, inp.value);
    box.appendChild(inp); box.appendChild(btn);
    wrap.appendChild(box);
  } else if (kind === 'model') {
    const row = document.createElement('span');
    row.className = 'model-row';
    const inp = document.createElement('input');
    inp.type = 'text';
    inp.value = p[field] || '';
    inp.setAttribute('list', 'models-' + p.key);
    inp.onchange = () => persistProviderField(p.key, field, inp.value);
    const dl = document.createElement('datalist');
    dl.id = 'models-' + p.key;
    row.appendChild(inp);
    row.appendChild(dl);
    if (!UNSUPPORTED_LIST_MODELS.has(p.provider)) {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'get-models-btn';
      btn.id = 'get-btn-' + p.key;
      btn.textContent = 'Get';
      const hasKey = (p.api_key && p.api_key.length > 0);
      if (hasKey) btn.classList.add('active');
      btn.onclick = () => fetchModels(p, inp, dl, btn, row);
      row.appendChild(btn);
    }
    wrap.appendChild(row);
  } else {
    const inp = document.createElement('input');
    inp.type = 'text';
    inp.value = p[field] || '';
    inp.onchange = () => persistProviderField(p.key, field, inp.value);
    wrap.appendChild(inp);
  }
  return wrap;
}
```

- [ ] **Step 3: Update the caller in `renderProviderCard` to pass `'model'` kind**

Still in `app.js`, find the call `card.appendChild(providerRow('Model', p, 'model', 'text'));` inside `renderProviderCard`. Replace with:

```js
  card.appendChild(providerRow('Model', p, 'model', 'model'));
```

- [ ] **Step 4: Add the unsupported set, `updateGetBtn`, and `fetchModels` helpers**

Add these three definitions to `app.js`. Place them near the other provider helpers (just below `persistProviderField`):

```js
// Providers whose backend doesn't implement provider.ModelLister.
// Kept in sync with server-side type assertions.
const UNSUPPORTED_LIST_MODELS = new Set(['zhipu', 'wenxin']);

// updateGetBtn is called from the api_key input's oninput handler so
// the Get button reacts live (no server roundtrip).
function updateGetBtn(key, apiKeyValue) {
  const btn = document.getElementById('get-btn-' + key);
  if (!btn) return;
  if (apiKeyValue && apiKeyValue.length > 0) {
    btn.classList.add('active');
  } else {
    btn.classList.remove('active');
  }
}

async function fetchModels(p, modelInput, datalist, btn, row) {
  if (!btn.classList.contains('active')) return;
  row.querySelectorAll('.inline-error').forEach(el => el.remove());
  const prevText = btn.textContent;
  btn.textContent = 'Loading…';
  btn.disabled = true;
  // Blur any active input so its onchange fires and persistProviderField
  // commits the current value. We do NOT iterate every input on the card:
  // the api_key field's displayed value may be the "••••" mask sentinel
  // that came back from /api/providers, and persisting that would overwrite
  // the real key in the in-memory doc. Rely on the browser's natural
  // blur→onchange→persist chain; if the persist hasn't round-tripped by the
  // time we fetch, the user retries. The race window is ~a few ms on
  // loopback.
  if (document.activeElement && typeof document.activeElement.blur === 'function') {
    document.activeElement.blur();
  }
  try {
    const r = await fetch('/api/providers/models', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({key: p.key}),
    });
    if (!r.ok) {
      const msg = await r.text();
      const err = document.createElement('span');
      err.className = 'inline-error';
      err.textContent = msg.trim();
      row.appendChild(err);
      btn.textContent = prevText;
      btn.disabled = false;
      return;
    }
    const body = await r.json();
    while (datalist.firstChild) datalist.removeChild(datalist.firstChild);
    for (const id of body.models || []) {
      const opt = document.createElement('option');
      opt.value = id;
      datalist.appendChild(opt);
    }
    btn.textContent = 'Got ' + (body.models || []).length;
    setTimeout(() => {
      btn.textContent = prevText;
      btn.disabled = false;
    }, 1000);
  } catch (e) {
    const err = document.createElement('span');
    err.className = 'inline-error';
    err.textContent = String(e);
    row.appendChild(err);
    btn.textContent = prevText;
    btn.disabled = false;
  }
}
```

- [ ] **Step 5: Validate JS syntax**

Run: `node --check cli/ui/webconfig/web/app.js`
Expected: no output (success).

- [ ] **Step 6: Build the binary to confirm embed still works**

Run: `go build -o bin/hermind ./cmd/hermind`
Expected: builds without error.

- [ ] **Step 7: Do NOT commit — part of the feat rollup**

---

## Task 7: Run the full suite

**Files:** none.

- [ ] **Step 1: Run all Go tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 2: Run `go vet`**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: If anything fails, stop and investigate**

Do not attempt the feat commit until the full suite is green. If an unrelated package now fails (e.g., a test that indirectly relied on env-var expansion), fix it in-place and re-run.

---

## Task 8: Commit the feat + manual smoke test

**Files:** none beyond what's already in the working tree.

- [ ] **Step 1: Inspect the staging set**

Run: `git status --short`
Expected:

```
 M cli/ui/webconfig/handlers.go
 M cli/ui/webconfig/server.go
 M cli/ui/webconfig/server_test.go
 M cli/ui/webconfig/web/app.css
 M cli/ui/webconfig/web/app.js
 M provider/provider.go
?? provider/anthropic/list_models.go
?? provider/anthropic/list_models_test.go
?? provider/openaicompat/list_models.go
?? provider/openaicompat/list_models_test.go
```

Anything else in the working tree (other than the untracked plan docs that were there before the session) is a sign of scope creep — stop and reconcile.

- [ ] **Step 2: Stage only the feat files**

Run:

```bash
git add provider/provider.go \
        provider/openaicompat/list_models.go \
        provider/openaicompat/list_models_test.go \
        provider/anthropic/list_models.go \
        provider/anthropic/list_models_test.go \
        cli/ui/webconfig/handlers.go \
        cli/ui/webconfig/server.go \
        cli/ui/webconfig/server_test.go \
        cli/ui/webconfig/web/app.css \
        cli/ui/webconfig/web/app.js
```

- [ ] **Step 3: Verify staged set**

Run: `git diff --staged --name-only`
Expected exactly the 10 paths above.

- [ ] **Step 4: Commit the feat**

```bash
git commit -m "$(cat <<'EOF'
feat(web-config): per-provider Get models button

Adds a provider.ModelLister optional interface. openaicompat.Client
implements it via GET {base_url}/models with Bearer auth — inherited
by DeepSeek, Kimi, Qwen, Minimax, OpenAI, OpenAICompat, and
OpenRouter. anthropic.Anthropic implements it with x-api-key +
anthropic-version headers against /v1/models. Zhipu and Wenxin
intentionally do not implement.

New POST /api/providers/models webconfig endpoint reads the in-memory
doc for credentials, instantiates the provider via factory.New, type-
asserts ModelLister, and returns {"models":[...]} with a 10s timeout.
Cross-origin requests return 403 and unsupported providers return
400 (defense in depth alongside the frontend allowlist).

The Providers section's model field now renders as an <input
list=...> + <datalist> combo with a trailing Get button. The button
is hidden for zhipu/wenxin, activates live as the api_key input
receives characters (oninput), and on click flushes pending edits
via persistProviderField for all four card fields before fetching.
Errors surface as .inline-error spans under the model row; success
populates the datalist and briefly shows "Got N" on the button.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5: Rebuild the binary for smoke testing**

Run: `go build -o bin/hermind ./cmd/hermind`

- [ ] **Step 6: Manual smoke checklist**

Launch: `./bin/hermind config --web` and walk this list:

1. Providers → an existing provider card shows the `model` field with a trailing `Get` button. The Get button text color is muted if the api_key is empty, amber if non-empty.
2. Backspace the api_key input → button goes muted within a keystroke (oninput reactivity).
3. Type a valid key → button goes amber.
4. Click Get (with a real key for openai/deepseek/etc.) → button briefly says `Loading…`, then `Got N`, then reverts. Clicking the model input shows a browser autocomplete dropdown with the fetched IDs. Type a prefix → filters.
5. Change provider type to `zhipu` → the Get button disappears from that card (re-render driven by the existing provider-type onchange).
6. Change provider type back to e.g. `openai`, delete api_key, click Get → the button should be inactive (no fetch attempt).
7. Set api_key to a wrong value and click Get → a red `.inline-error` span appears under the model row with the provider's error string (e.g. `401`).
8. Add a new provider via `+ Add provider` → new card lacks an api_key → Get is muted → paste a key → live activates.
9. Click Save → status goes green: `saved — restart hermind to apply`.
10. `env:FOO` literal: open `~/.hermind/config.yaml`, find any value that starts with `env:` from a prior version; load the web UI → value is shown verbatim (not resolved). Paste a real key, Save.

Report any smoke failures before declaring done.

- [ ] **Step 7: Confirm the log**

Run: `git log -3 --oneline`
Expected:

```
<sha> feat(web-config): per-provider Get models button
<sha> refactor(config): drop env:VAR api-key indirection
<sha> docs: spec for literal api keys + per-provider model fetch
```

---

## Rollback

Two independent reverts:

- `git revert <feat sha>` removes the model-fetch UI and the endpoint. Configs continue to work.
- `git revert <refactor sha>` restores `env:VAR` expansion. Must revert in this order if both are rolled back (feat depends on factory.New which doesn't care about expansion, but the CHANGELOG narrative assumes both or neither).

No data migration on either revert.
