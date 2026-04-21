# Web Search Multi-Provider Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade `tool/web/search.go` from a single Exa-only implementation to a pluggable multi-provider structure supporting DuckDuckGo, Tavily, Brave, and Exa behind a unified result schema with a 60s LRU cache.

**Architecture:** `SearchProvider` interface with four implementations living as flat files in `tool/web/` (following the existing `tool/browser/` pattern). A `searchDispatcher` resolves the active provider by explicit config or internal priority (Tavily > Brave > Exa > DuckDuckGo), checks a 60s LRU cache (max 128 entries) keyed by `providerID|query|num`, and normalizes every provider's native response into `{query, provider, results:[{title, url, snippet, published_date?, score?}]}`. Providers return Go errors; the dispatcher converts them into `tool.ToolError` strings so the LLM sees structured output.

**Tech Stack:** Go 1.23 stdlib (net/http, net/url, sync, time, encoding/json), `github.com/stretchr/testify/assert`+`/require` (existing test deps), `github.com/PuerkitoBio/goquery` (new dep — DDG HTML parsing only).

**Spec reference:** `docs/superpowers/specs/2026-04-21-websearch-port-design.md`.

**Pre-flight orientation for the engineer:**
- The project is a Go-based agent runtime. Tool handlers live under `tool/` and are registered into a central `tool.Registry`.
- Config is YAML (`config/config.go` + `yaml` tags, parsed by `gopkg.in/yaml.v3`). The Config struct is passed through `cli/repl.go` at startup.
- Descriptors in `config/descriptor/*.go` publish config schema to the web UI. One file per top-level section; each uses `init()` to call `Register`.
- Go error handling: tool handlers return `(string, error)`. Existing tools generally return `(tool.ToolError(msg), nil)` for user-facing errors rather than raw `error` — this plan keeps that convention.
- Tests use `stretchr/testify` (`assert` + `require`). `httptest.NewServer` is the standard way to test HTTP clients. No gomock.
- Providers should accept an `endpoint` string ctor param so tests can inject `httptest.Server.URL`.

---

## File Structure

**Create (new files):**
```
tool/web/search_provider.go            SearchProvider interface + SearchResult struct + shared constants
tool/web/search_dispatcher.go          searchCache + searchDispatcher + resolveProvider + handler factory
tool/web/search_exa.go                 Exa provider impl (migrated from current search.go)
tool/web/search_tavily.go              Tavily provider impl
tool/web/search_brave.go               Brave Search provider impl
tool/web/search_ddg.go                 DuckDuckGo provider impl (adds goquery dep)
tool/web/search_dispatcher_test.go     cache + resolveProvider + handler tests
tool/web/search_exa_test.go            Exa provider tests (migrated)
tool/web/search_tavily_test.go         Tavily provider tests
tool/web/search_brave_test.go          Brave provider tests
tool/web/search_ddg_test.go            DDG provider tests
config/descriptor/web.go               descriptor for web.search.*
config/descriptor/web_test.go          descriptor registration tests
docs/smoke/web-search.md               manual verification guide
```

**Modify:**
```
config/config.go                       add WebConfig + SearchConfig + SearchProvidersConfig + ProviderKeyConfig; wire into Config struct
config/loader_test.go                  (if exists) add a YAML decode test asserting the new Web section is populated
tool/web/search.go                     strip Exa internals, keep webSearchSchema + buildWebSearchHandler that delegates to dispatcher
tool/web/register.go                   new RegisterAll(reg, Options) signature
tool/web/web_test.go                   remove the three Exa-specific tests (moved to search_exa_test.go); keep web_fetch + web_extract tests
cli/repl.go:128-130                    call new RegisterAll(reg, Options) signature
CHANGELOG.md                           additive entry: new providers + cache + new dep
go.mod / go.sum                        add github.com/PuerkitoBio/goquery
```

**Unchanged:**
```
tool/web/fetch.go
tool/web/extract.go
```

---

## Task 1: Config Types

Add the `WebConfig` / `SearchConfig` / `SearchProvidersConfig` / `ProviderKeyConfig` structs to `config/config.go` and wire a `Web WebConfig` field into the top-level `Config` struct.

**Files:**
- Modify: `config/config.go`
- Modify: `config/loader_test.go` (add new test)

- [ ] **Step 1: Read the existing Config struct to find the insertion point**

Run:
```bash
grep -n "type Config struct" config/config.go
grep -n "Skills.*SkillsConfig" config/config.go
```
Expected: Config struct starts around line 5; `Skills` is around line 21 (the last yaml-tagged field).

- [ ] **Step 2: Write the failing test**

Append to `config/loader_test.go` (if the file doesn't exist, create it):

```go
func TestLoadWebSearchConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
web:
  search:
    provider: tavily
    providers:
      tavily:
        api_key: "tav-123"
      brave:
        api_key: "brv-456"
      exa:
        api_key: "exa-789"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Search.Provider != "tavily" {
		t.Errorf("Provider = %q, want tavily", cfg.Web.Search.Provider)
	}
	if cfg.Web.Search.Providers.Tavily.APIKey != "tav-123" {
		t.Errorf("Tavily.APIKey = %q, want tav-123", cfg.Web.Search.Providers.Tavily.APIKey)
	}
	if cfg.Web.Search.Providers.Brave.APIKey != "brv-456" {
		t.Errorf("Brave.APIKey = %q, want brv-456", cfg.Web.Search.Providers.Brave.APIKey)
	}
	if cfg.Web.Search.Providers.Exa.APIKey != "exa-789" {
		t.Errorf("Exa.APIKey = %q, want exa-789", cfg.Web.Search.Providers.Exa.APIKey)
	}
}
```

If `config/loader_test.go` has no `path/filepath` / `os` imports yet, add them.

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./config/ -run TestLoadWebSearchConfig -v`
Expected: FAIL with `cfg.Web` undefined (or similar compile error).

- [ ] **Step 4: Add the types and wire into Config**

Append to `config/config.go` (after the last config type, before `Default()`):

```go
// WebConfig holds configuration for the `web_*` tool family.
// Firecrawl (used by web_extract) continues to read FIRECRAWL_API_KEY
// directly and is not represented here.
type WebConfig struct {
	Search SearchConfig `yaml:"search,omitempty"`
}

// SearchConfig configures the web_search tool's provider abstraction.
// Provider selects the active backend; empty string enables auto-selection
// by priority (tavily > brave > exa > ddg).
type SearchConfig struct {
	Provider  string                `yaml:"provider,omitempty"`
	Providers SearchProvidersConfig `yaml:"providers,omitempty"`
}

// SearchProvidersConfig holds per-provider credentials. DDG does not
// require credentials and therefore has no sub-node.
type SearchProvidersConfig struct {
	Tavily ProviderKeyConfig `yaml:"tavily,omitempty"`
	Brave  ProviderKeyConfig `yaml:"brave,omitempty"`
	Exa    ProviderKeyConfig `yaml:"exa,omitempty"`
}

// ProviderKeyConfig is the shared shape for an API-key-only provider.
type ProviderKeyConfig struct {
	APIKey string `yaml:"api_key,omitempty"`
}
```

Add the field to the Config struct — insert after `Skills SkillsConfig` (near line 21):

```go
	Web               WebConfig                 `yaml:"web,omitempty"`
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./config/ -run TestLoadWebSearchConfig -v`
Expected: PASS.

Also run the full config test suite to make sure nothing else broke:
Run: `go test ./config/...`
Expected: PASS (all).

- [ ] **Step 6: Commit**

```bash
git add config/config.go config/loader_test.go
git commit -m "feat(config): web.search.* config types"
```

---

## Task 2: Web Descriptor

Add `config/descriptor/web.go` publishing the new `web.search.*` fields to the UI and `config/descriptor/web_test.go` verifying registration.

**Files:**
- Create: `config/descriptor/web.go`
- Create: `config/descriptor/web_test.go`

- [ ] **Step 1: Write the failing test**

Create `config/descriptor/web_test.go`:

```go
package descriptor

import "testing"

func TestWebSectionRegistered(t *testing.T) {
	s, ok := Get("web")
	if !ok {
		t.Fatal("Get(\"web\") returned ok=false — did web.go init() register?")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestWebSearchProviderEnum(t *testing.T) {
	s, _ := Get("web")
	var p *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "search.provider" {
			p = &s.Fields[i]
			break
		}
	}
	if p == nil {
		t.Fatal("search.provider field missing")
	}
	if p.Kind != FieldEnum {
		t.Errorf("search.provider.Kind = %s, want enum", p.Kind)
	}
	want := map[string]bool{"": true, "tavily": true, "brave": true, "exa": true, "ddg": true}
	got := map[string]bool{}
	for _, v := range p.Enum {
		got[v] = true
	}
	for v := range want {
		if !got[v] {
			t.Errorf("search.provider.Enum missing %q, got %v", v, p.Enum)
		}
	}
}

func TestWebProviderAPIKeysAreSecrets(t *testing.T) {
	s, _ := Get("web")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range []string{
		"search.providers.tavily.api_key",
		"search.providers.brave.api_key",
		"search.providers.exa.api_key",
	} {
		f, ok := byName[name]
		if !ok {
			t.Errorf("field %q missing", name)
			continue
		}
		if f.Kind != FieldSecret {
			t.Errorf("%s.Kind = %s, want secret", name, f.Kind)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./config/descriptor/ -run TestWebSection -v`
Expected: FAIL with `Get("web") returned ok=false`.

- [ ] **Step 3: Write the descriptor**

Create `config/descriptor/web.go`:

```go
package descriptor

// Web mirrors config.WebConfig. Currently only hosts the web_search
// provider abstraction. Firecrawl (used by web_extract) reads
// FIRECRAWL_API_KEY from the environment and has no UI field.
//
// Dotted field names like "search.provider" and
// "search.providers.tavily.api_key" rely on the dotted-path
// infrastructure in ConfigSection.tsx, state.ts (edit/config-field
// reducer), and api/handlers_config.go (walkPath helper).
func init() {
	Register(Section{
		Key:     "web",
		Label:   "Web tools",
		Summary: "Web search provider configuration. DuckDuckGo is the keyless fallback and always available.",
		GroupID: "advanced",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "search.provider",
				Label: "Search provider",
				Help:  "Leave blank to auto-select by priority (Tavily > Brave > Exa > DuckDuckGo).",
				Kind:  FieldEnum,
				Enum:  []string{"", "tavily", "brave", "exa", "ddg"},
			},
			{
				Name:  "search.providers.tavily.api_key",
				Label: "Tavily API key",
				Kind:  FieldSecret,
				Help:  "Env var TAVILY_API_KEY overrides this value at runtime.",
			},
			{
				Name:  "search.providers.brave.api_key",
				Label: "Brave Search API key",
				Kind:  FieldSecret,
				Help:  "Env var BRAVE_API_KEY overrides this value at runtime.",
			},
			{
				Name:  "search.providers.exa.api_key",
				Label: "Exa API key",
				Kind:  FieldSecret,
				Help:  "Env var EXA_API_KEY overrides this value at runtime.",
			},
		},
	})
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./config/descriptor/ -v`
Expected: PASS (all, including existing browser/memory/etc tests).

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/web.go config/descriptor/web_test.go
git commit -m "feat(config/descriptor): Web section descriptor"
```

---

## Task 3: SearchProvider Interface + Types

Define the `SearchProvider` interface, `SearchResult` struct, shared request-arg schema constants, and the `Options` struct consumed by `RegisterAll`.

**Files:**
- Create: `tool/web/search_provider.go`

- [ ] **Step 1: Write the (compile-only) failing check**

Run: `ls tool/web/search_provider.go`
Expected: FAIL with "No such file or directory".

(No runtime test yet — this is a shared types file. Downstream tasks will provide behavioural tests.)

- [ ] **Step 2: Create the file with interface + types**

Create `tool/web/search_provider.go`:

```go
// tool/web/search_provider.go
package web

import (
	"context"
	"time"
)

// httpTimeout is the per-request timeout every provider applies to its
// outbound HTTP call.
const httpTimeout = 30 * time.Second

// SearchProvider is the contract every web_search backend implements.
// Implementations handle their own HTTP client, auth, and response
// parsing. They return normalized SearchResult values; the dispatcher
// does not inspect provider-specific raw responses.
type SearchProvider interface {
	// ID returns the stable provider identifier used in config, cache
	// keys, and error messages. One of "ddg", "tavily", "brave", "exa".
	ID() string

	// Configured returns true when the provider has everything it needs
	// to run a search (typically an API key). DDG always returns true.
	Configured() bool

	// Search performs a single query against the provider's backend.
	// n is already clamped to [1, 20] by the dispatcher. Returns a
	// provider-agnostic list of normalized results.
	Search(ctx context.Context, q string, n int) ([]SearchResult, error)
}

// SearchResult is the normalized shape emitted to the LLM. All providers
// map their native fields into this structure. Score is a pointer so
// providers without a relevance score (DDG, Brave) omit the field
// rather than emit 0.0.
type SearchResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Snippet       string   `json:"snippet"`
	PublishedDate string   `json:"published_date,omitempty"`
	Score         *float64 `json:"score,omitempty"`
}

// Options is the flat parameter bundle consumed by RegisterAll. It is
// populated by cli/repl.go (or cli/engine_deps.go post-Phase 1) from
// the parsed config.Config plus environment variables.
type Options struct {
	// SearchProvider pins the active provider ID. Empty string enables
	// auto-selection by priority.
	SearchProvider string
	// Tavily/Brave/Exa API keys. Empty string means "not configured"
	// (the provider's Configured() returns false and auto-select skips
	// it). DDG has no key.
	TavilyAPIKey string
	BraveAPIKey  string
	ExaAPIKey    string
	// FirecrawlAPIKey is forwarded to the existing web_extract tool
	// registration. Not consumed by the search dispatcher.
	FirecrawlAPIKey string
}
```

- [ ] **Step 3: Run compile check**

Run: `go build ./tool/web/...`
Expected: PASS (the file compiles in isolation; no usages yet).

- [ ] **Step 4: Commit**

```bash
git add tool/web/search_provider.go
git commit -m "feat(tool/web): SearchProvider interface + Options"
```

---

## Task 4: Search Cache

Implement the 60s-TTL / 128-entry LRU `searchCache` in `tool/web/search_dispatcher.go`. Tests assert Get/Set, expiry, and LRU eviction behaviour.

**Files:**
- Create: `tool/web/search_dispatcher.go` (cache portion only this task)
- Create: `tool/web/search_dispatcher_test.go`

- [ ] **Step 1: Write the failing tests**

Create `tool/web/search_dispatcher_test.go`:

```go
// tool/web/search_dispatcher_test.go
package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSearchCache_GetMissReturnsZero(t *testing.T) {
	c := newSearchCache(4, time.Second)
	v, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Nil(t, v)
}

func TestSearchCache_SetThenGetReturnsValue(t *testing.T) {
	c := newSearchCache(4, time.Second)
	results := []SearchResult{{Title: "A", URL: "https://a"}}
	c.Set("k1", results)
	v, ok := c.Get("k1")
	assert.True(t, ok)
	assert.Equal(t, results, v)
}

func TestSearchCache_ExpiryRemovesEntry(t *testing.T) {
	c := newSearchCache(4, 10*time.Millisecond)
	c.Set("k1", []SearchResult{{Title: "A"}})
	time.Sleep(20 * time.Millisecond)
	v, ok := c.Get("k1")
	assert.False(t, ok)
	assert.Nil(t, v)
}

func TestSearchCache_LRUEviction(t *testing.T) {
	c := newSearchCache(2, time.Second)
	c.Set("k1", []SearchResult{{Title: "A"}})
	c.Set("k2", []SearchResult{{Title: "B"}})
	// Touch k1 to make it MRU
	_, _ = c.Get("k1")
	// Insert k3 — k2 should be evicted (LRU)
	c.Set("k3", []SearchResult{{Title: "C"}})

	_, ok := c.Get("k1")
	assert.True(t, ok, "k1 should remain (was MRU)")
	_, ok = c.Get("k2")
	assert.False(t, ok, "k2 should be evicted (was LRU)")
	_, ok = c.Get("k3")
	assert.True(t, ok, "k3 should remain")
}

func TestSearchCache_ConcurrentAccess(t *testing.T) {
	c := newSearchCache(100, time.Second)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				key := string(rune('a' + (id+j)%26))
				c.Set(key, []SearchResult{{Title: key}})
				_, _ = c.Get(key)
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// No panic / race means pass. (Run with -race to verify.)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tool/web/ -run TestSearchCache -v`
Expected: FAIL with `newSearchCache undefined`.

- [ ] **Step 3: Write the cache implementation**

Create `tool/web/search_dispatcher.go`:

```go
// tool/web/search_dispatcher.go
package web

import (
	"sync"
	"time"
)

// cacheEntry is one slot in searchCache. expiresAt is evaluated lazily
// on Get — no background reaper goroutine.
type cacheEntry struct {
	value     []SearchResult
	expiresAt time.Time
}

// searchCache is a bounded, TTL-aware LRU. All operations take a single
// mutex. order is the eviction queue: index 0 is the LRU slot, the last
// index is the MRU slot.
//
// The implementation uses a slice + linear search rather than a doubly
// linked list. At the configured cap (128 entries) the O(N) cost is
// dwarfed by the HTTP round-trip it saves.
type searchCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	order   []string
	maxSize int
	ttl     time.Duration
}

// newSearchCache returns a cache with the given capacity and TTL.
func newSearchCache(maxSize int, ttl time.Duration) *searchCache {
	return &searchCache{
		entries: make(map[string]cacheEntry, maxSize),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get returns the cached value if present and not expired. Expired
// entries are removed on access. Accessing a live entry moves it to
// MRU position.
func (c *searchCache) Get(key string) ([]SearchResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		c.removeLocked(key)
		return nil, false
	}
	c.touchLocked(key)
	return e.value, true
}

// Set inserts or replaces a value. When the cache is full, the LRU
// entry is evicted before insertion.
func (c *searchCache) Set(key string, value []SearchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; exists {
		c.entries[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
		c.touchLocked(key)
		return
	}
	if len(c.entries) >= c.maxSize && len(c.order) > 0 {
		c.removeLocked(c.order[0])
	}
	c.entries[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.order = append(c.order, key)
}

func (c *searchCache) removeLocked(key string) {
	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

func (c *searchCache) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tool/web/ -run TestSearchCache -v -race`
Expected: PASS (5/5, no race warnings).

- [ ] **Step 5: Commit**

```bash
git add tool/web/search_dispatcher.go tool/web/search_dispatcher_test.go
git commit -m "feat(tool/web): searchCache — 60s LRU with TTL"
```

---

## Task 5: Exa Provider

Migrate the existing Exa implementation from `tool/web/search.go` into `tool/web/search_exa.go` behind the new `SearchProvider` interface. Move existing Exa tests from `web_test.go` to `search_exa_test.go`.

**Files:**
- Create: `tool/web/search_exa.go`
- Create: `tool/web/search_exa_test.go`
- Modify: `tool/web/web_test.go` (remove three Exa-specific tests)

- [ ] **Step 1: Write the failing test**

Create `tool/web/search_exa_test.go`:

```go
// tool/web/search_exa_test.go
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExaProvider_HappyPath(t *testing.T) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query      string `json:"query"`
			NumResults int    `json:"numResults"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		captured = req.Query
		assert.Equal(t, 5, req.NumResults)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"title": "Go", "url": "https://go.dev", "text": "Programming language", "publishedDate": "2024-01", "score": 0.9},
				{"title": "Effective Go", "url": "https://go.dev/doc/effective_go"},
			},
		})
	}))
	defer srv.Close()

	p := newExaProvider("test-key", srv.URL)
	results, err := p.Search(context.Background(), "golang", 5)
	require.NoError(t, err)
	assert.Equal(t, "golang", captured)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-01", results[0].PublishedDate)
	require.NotNil(t, results[0].Score)
	assert.InDelta(t, 0.9, *results[0].Score, 0.001)
}

func TestExaProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newExaProvider("bad-key", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 401")
}

func TestExaProvider_ConfiguredRequiresKey(t *testing.T) {
	t.Setenv("EXA_API_KEY", "")
	assert.False(t, newExaProvider("", "").Configured())
	assert.True(t, newExaProvider("k", "").Configured())
}

func TestExaProvider_ConfiguredUsesEnvFallback(t *testing.T) {
	t.Setenv("EXA_API_KEY", "env-key")
	p := newExaProvider("", "")
	assert.True(t, p.Configured())
}

func TestExaProvider_ID(t *testing.T) {
	assert.Equal(t, "exa", newExaProvider("k", "").ID())
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tool/web/ -run TestExaProvider -v`
Expected: FAIL with `newExaProvider undefined`.

- [ ] **Step 3: Write the Exa provider**

Create `tool/web/search_exa.go`:

```go
// tool/web/search_exa.go
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// exaEndpoint is the production Exa search endpoint. Named distinct
// from the legacy `exaDefaultURL` still living in search.go so both
// can coexist until Task 10 strips the legacy block.
const exaEndpoint = "https://api.exa.ai/search"

type exaProvider struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

// newExaProvider constructs an Exa-backed SearchProvider. If apiKey is
// empty, the EXA_API_KEY environment variable is used as a fallback
// during Configured() checks. The endpoint override is for tests; an
// empty string resolves to exaEndpoint at request time.
func newExaProvider(apiKey, endpoint string) *exaProvider {
	return &exaProvider{
		apiKey:   apiKey,
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *exaProvider) ID() string { return "exa" }

func (p *exaProvider) Configured() bool {
	return p.resolvedKey() != ""
}

func (p *exaProvider) resolvedKey() string {
	if p.apiKey != "" {
		return p.apiKey
	}
	return os.Getenv("EXA_API_KEY")
}

type exaRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
}

type exaResponse struct {
	Results []exaResultItem `json:"results"`
}

type exaResultItem struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Text          string  `json:"text,omitempty"`
	PublishedDate string  `json:"publishedDate,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

func (p *exaProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	key := p.resolvedKey()
	if key == "" {
		return nil, fmt.Errorf("api key missing")
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = exaEndpoint
	}

	body, _ := json.Marshal(exaRequest{Query: q, NumResults: n})
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var out exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	results := make([]SearchResult, 0, len(out.Results))
	for _, r := range out.Results {
		item := SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Text,
			PublishedDate: r.PublishedDate,
		}
		if r.Score != 0 {
			s := r.Score
			item.Score = &s
		}
		results = append(results, item)
	}
	return results, nil
}
```

- [ ] **Step 4: Remove the old Exa tests from web_test.go**

Open `tool/web/web_test.go` and delete these three test functions (together with their imports if they become unused):
- `TestWebSearchHappyPath` (lines ~106–137)
- `TestWebSearchRequiresQuery` (lines ~139–145)
- `TestWebSearchRejectsMissingKey` (lines ~147–153)

Keep `TestWebFetchHappyPath`, `TestWebFetchRejectsMissingURL`, `TestWebFetchHandlesNon200`, `TestWebFetchTruncatesLargeResponses`, `TestWebExtractHappyPath`, `TestWebExtractRejectsBadFormat`, `TestWebExtractHandlesFailureResponse`.

Note: `web_test.go` still references `exaSearchRequest`, `exaSearchResponse`, `exaResult`, `newWebSearchHandler`, and `firecrawlRequest`/`firecrawlResponse`/`webExtractResult`. After deleting the three Exa tests, only the firecrawl/fetch identifiers remain referenced. The compile won't break yet because the Exa types are still defined in `search.go` (we clean those up in Task 10).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./tool/web/ -run TestExaProvider -v`
Expected: PASS (5/5).

Run: `go test ./tool/web/ -v`
Expected: PASS (all remaining web_test.go tests + Exa provider tests).

- [ ] **Step 6: Commit**

```bash
git add tool/web/search_exa.go tool/web/search_exa_test.go tool/web/web_test.go
git commit -m "feat(tool/web): Exa provider behind SearchProvider interface"
```

---

## Task 6: Tavily Provider

Implement Tavily — POST JSON API with api_key in body, score + published_date in results.

**Files:**
- Create: `tool/web/search_tavily.go`
- Create: `tool/web/search_tavily_test.go`

- [ ] **Step 1: Write the failing test**

Create `tool/web/search_tavily_test.go`:

```go
// tool/web/search_tavily_test.go
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTavilyProvider_HappyPath(t *testing.T) {
	var capturedKey, capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			APIKey     string `json:"api_key"`
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		capturedKey = req.APIKey
		capturedQuery = req.Query
		assert.Equal(t, 7, req.MaxResults)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query": "golang",
			"results": []map[string]any{
				{"title": "Go", "url": "https://go.dev", "content": "Programming language", "score": 0.85, "published_date": "2024-03-01"},
				{"title": "Tour", "url": "https://go.dev/tour"},
			},
		})
	}))
	defer srv.Close()

	p := newTavilyProvider("tav-key", srv.URL)
	results, err := p.Search(context.Background(), "golang", 7)
	require.NoError(t, err)
	assert.Equal(t, "tav-key", capturedKey)
	assert.Equal(t, "golang", capturedQuery)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-03-01", results[0].PublishedDate)
	require.NotNil(t, results[0].Score)
	assert.InDelta(t, 0.85, *results[0].Score, 0.001)
	assert.Nil(t, results[1].Score, "missing score must serialize as nil pointer")
}

func TestTavilyProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newTavilyProvider("bad", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 401")
}

func TestTavilyProvider_Configured(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	assert.False(t, newTavilyProvider("", "").Configured())
	assert.True(t, newTavilyProvider("k", "").Configured())

	t.Setenv("TAVILY_API_KEY", "env-key")
	assert.True(t, newTavilyProvider("", "").Configured())
}

func TestTavilyProvider_ID(t *testing.T) {
	assert.Equal(t, "tavily", newTavilyProvider("k", "").ID())
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tool/web/ -run TestTavilyProvider -v`
Expected: FAIL with `newTavilyProvider undefined`.

- [ ] **Step 3: Write the Tavily provider**

Create `tool/web/search_tavily.go`:

```go
// tool/web/search_tavily.go
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const tavilyDefaultURL = "https://api.tavily.com/search"

type tavilyProvider struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func newTavilyProvider(apiKey, endpoint string) *tavilyProvider {
	return &tavilyProvider{
		apiKey:   apiKey,
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *tavilyProvider) ID() string { return "tavily" }

func (p *tavilyProvider) Configured() bool {
	return p.resolvedKey() != ""
}

func (p *tavilyProvider) resolvedKey() string {
	if p.apiKey != "" {
		return p.apiKey
	}
	return os.Getenv("TAVILY_API_KEY")
}

type tavilyRequest struct {
	APIKey            string `json:"api_key"`
	Query             string `json:"query"`
	MaxResults        int    `json:"max_results"`
	IncludeAnswer     bool   `json:"include_answer"`
	IncludeRawContent bool   `json:"include_raw_content"`
}

type tavilyResponse struct {
	Query   string              `json:"query"`
	Results []tavilyResultItem `json:"results"`
}

type tavilyResultItem struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score,omitempty"`
	PublishedDate string  `json:"published_date,omitempty"`
}

func (p *tavilyProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	key := p.resolvedKey()
	if key == "" {
		return nil, fmt.Errorf("api key missing")
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = tavilyDefaultURL
	}

	body, _ := json.Marshal(tavilyRequest{
		APIKey:            key,
		Query:             q,
		MaxResults:        n,
		IncludeAnswer:     false,
		IncludeRawContent: false,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var out tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	results := make([]SearchResult, 0, len(out.Results))
	for _, r := range out.Results {
		item := SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Content,
			PublishedDate: r.PublishedDate,
		}
		if r.Score != 0 {
			s := r.Score
			item.Score = &s
		}
		results = append(results, item)
	}
	return results, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tool/web/ -run TestTavilyProvider -v`
Expected: PASS (4/4).

- [ ] **Step 5: Commit**

```bash
git add tool/web/search_tavily.go tool/web/search_tavily_test.go
git commit -m "feat(tool/web): Tavily provider"
```

---

## Task 7: Brave Provider

Implement Brave Search — GET query, `X-Subscription-Token` header, nested `web.results` response shape.

**Files:**
- Create: `tool/web/search_brave.go`
- Create: `tool/web/search_brave_test.go`

- [ ] **Step 1: Write the failing test**

Create `tool/web/search_brave_test.go`:

```go
// tool/web/search_brave_test.go
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBraveProvider_HappyPath(t *testing.T) {
	var capturedQuery, capturedCount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "brv-key", r.Header.Get("X-Subscription-Token"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		capturedQuery = r.URL.Query().Get("q")
		capturedCount = r.URL.Query().Get("count")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"results": []map[string]any{
					{"title": "Go", "url": "https://go.dev", "description": "Programming language", "page_age": "2024-02-01"},
					{"title": "Docs", "url": "https://go.dev/doc", "description": "Docs"},
				},
			},
		})
	}))
	defer srv.Close()

	p := newBraveProvider("brv-key", srv.URL)
	results, err := p.Search(context.Background(), "golang", 3)
	require.NoError(t, err)
	assert.Equal(t, "golang", capturedQuery)
	assert.Equal(t, "3", capturedCount)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-02-01", results[0].PublishedDate)
	assert.Nil(t, results[0].Score, "Brave has no score")
	assert.Equal(t, "", results[1].PublishedDate, "missing page_age → empty")
}

func TestBraveProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := newBraveProvider("bad", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 403")
}

func TestBraveProvider_Configured(t *testing.T) {
	t.Setenv("BRAVE_API_KEY", "")
	assert.False(t, newBraveProvider("", "").Configured())
	assert.True(t, newBraveProvider("k", "").Configured())

	t.Setenv("BRAVE_API_KEY", "env-key")
	assert.True(t, newBraveProvider("", "").Configured())
}

func TestBraveProvider_ID(t *testing.T) {
	assert.Equal(t, "brave", newBraveProvider("k", "").ID())
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tool/web/ -run TestBraveProvider -v`
Expected: FAIL with `newBraveProvider undefined`.

- [ ] **Step 3: Write the Brave provider**

Create `tool/web/search_brave.go`:

```go
// tool/web/search_brave.go
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

const braveDefaultURL = "https://api.search.brave.com/res/v1/web/search"

type braveProvider struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func newBraveProvider(apiKey, endpoint string) *braveProvider {
	return &braveProvider{
		apiKey:   apiKey,
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *braveProvider) ID() string { return "brave" }

func (p *braveProvider) Configured() bool {
	return p.resolvedKey() != ""
}

func (p *braveProvider) resolvedKey() string {
	if p.apiKey != "" {
		return p.apiKey
	}
	return os.Getenv("BRAVE_API_KEY")
}

type braveResponse struct {
	Web struct {
		Results []braveResultItem `json:"results"`
	} `json:"web"`
}

type braveResultItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	PageAge     string `json:"page_age,omitempty"`
}

func (p *braveProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	key := p.resolvedKey()
	if key == "" {
		return nil, fmt.Errorf("api key missing")
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = braveDefaultURL
	}

	params := url.Values{}
	params.Set("q", q)
	params.Set("count", strconv.Itoa(n))

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", key)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var out braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	results := make([]SearchResult, 0, len(out.Web.Results))
	for _, r := range out.Web.Results {
		results = append(results, SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Description,
			PublishedDate: r.PageAge,
		})
	}
	return results, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tool/web/ -run TestBraveProvider -v`
Expected: PASS (4/4).

- [ ] **Step 5: Commit**

```bash
git add tool/web/search_brave.go tool/web/search_brave_test.go
git commit -m "feat(tool/web): Brave Search provider"
```

---

## Task 8: DuckDuckGo Provider

Implement DuckDuckGo via HTML scraping of `https://html.duckduckgo.com/html/`. Adds `github.com/PuerkitoBio/goquery` to go.mod. Handles the `uddg` outbound-link encoding and detects CAPTCHA throttling.

**Files:**
- Create: `tool/web/search_ddg.go`
- Create: `tool/web/search_ddg_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add goquery to go.mod**

Run: `go get github.com/PuerkitoBio/goquery@v1.9.2`
Expected: go.mod + go.sum updated.

- [ ] **Step 2: Write the failing test**

Create `tool/web/search_ddg_test.go`:

```go
// tool/web/search_ddg_test.go
package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ddgHTMLFixture = `
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
    <a class="result__a" rel="nofollow" href="/l/?uddg=https%3A%2F%2Fgo.dev&rut=xx">Go programming language</a>
  </h2>
  <a class="result__snippet" href="/l/?uddg=https%3A%2F%2Fgo.dev">The Go programming language is an open source project.</a>
</div>
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
    <a class="result__a" rel="nofollow" href="/l/?uddg=https%3A%2F%2Fgo.dev%2Fdoc&rut=yy">Go documentation</a>
  </h2>
  <a class="result__snippet">Documentation for the Go programming language.</a>
</div>
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title">
    <a class="result__a" rel="nofollow" href="https://example.com/direct">Direct absolute URL</a>
  </h2>
  <a class="result__snippet">Already absolute.</a>
</div>
`

func TestDDGProvider_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		assert.Equal(t, "golang", form.Get("q"))

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(ddgHTMLFixture))
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	results, err := p.Search(context.Background(), "golang", 10)
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, "Go programming language", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Contains(t, results[0].Snippet, "open source project")
	assert.Nil(t, results[0].Score)

	assert.Equal(t, "Go documentation", results[1].Title)
	assert.Equal(t, "https://go.dev/doc", results[1].URL)

	// Absolute URL passthrough (no uddg wrapper)
	assert.Equal(t, "https://example.com/direct", results[2].URL)
}

func TestDDGProvider_CAPTCHAReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div class="anomaly-modal">Sorry, please verify.</div></body></html>`))
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	_, err := p.Search(context.Background(), "q", 10)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "rate limited")
}

func TestDDGProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	_, err := p.Search(context.Background(), "q", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 502")
}

func TestDDGProvider_AlwaysConfigured(t *testing.T) {
	assert.True(t, newDDGProvider("").Configured())
}

func TestDDGProvider_ID(t *testing.T) {
	assert.Equal(t, "ddg", newDDGProvider("").ID())
}

func TestDDGProvider_RespectsNResults(t *testing.T) {
	// Build fixture with 8 results
	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(`
<div class="result results_links results_links_deep web-result">
  <h2 class="result__title"><a class="result__a" href="https://x/`)
		sb.WriteString("a")
		sb.WriteString(`">T</a></h2>
  <a class="result__snippet">S</a>
</div>`)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sb.String()))
	}))
	defer srv.Close()

	p := newDDGProvider(srv.URL)
	results, err := p.Search(context.Background(), "q", 3)
	require.NoError(t, err)
	assert.Len(t, results, 3, "should clip to n=3")
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./tool/web/ -run TestDDGProvider -v`
Expected: FAIL with `newDDGProvider undefined`.

- [ ] **Step 4: Write the DDG provider**

Create `tool/web/search_ddg.go`:

```go
// tool/web/search_ddg.go
package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const ddgDefaultURL = "https://html.duckduckgo.com/html/"

type ddgProvider struct {
	endpoint string
	client   *http.Client
}

func newDDGProvider(endpoint string) *ddgProvider {
	return &ddgProvider{
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *ddgProvider) ID() string { return "ddg" }

// Configured returns true unconditionally: DuckDuckGo's HTML endpoint
// does not require an API key.
func (p *ddgProvider) Configured() bool { return true }

func (p *ddgProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = ddgDefaultURL
	}
	form := url.Values{}
	form.Set("q", q)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Give DDG a recognizable UA to reduce false-positive throttling
	req.Header.Set("User-Agent", "hermind/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	// Rate-limit detection: DDG returns a page containing the string
	// "anomaly" (class="anomaly-modal") when throttling.
	if strings.Contains(strings.ToLower(doc.Text()), "anomaly") {
		return nil, fmt.Errorf("rate limited")
	}

	results := make([]SearchResult, 0, n)
	doc.Find(".result").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		if len(results) >= n {
			return false
		}
		link := sel.Find(".result__a").First()
		title := strings.TrimSpace(link.Text())
		href, _ := link.Attr("href")
		actual := decodeDDGLink(href)
		if title == "" || actual == "" {
			return true
		}
		snippet := strings.TrimSpace(sel.Find(".result__snippet").First().Text())
		results = append(results, SearchResult{
			Title:   title,
			URL:     actual,
			Snippet: snippet,
		})
		return true
	})
	return results, nil
}

// decodeDDGLink extracts the real destination from DDG's /l/?uddg=...
// wrapper. If raw is already an absolute URL, it is returned as-is.
func decodeDDGLink(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if dest := u.Query().Get("uddg"); dest != "" {
		return dest
	}
	return ""
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./tool/web/ -run TestDDGProvider -v`
Expected: PASS (6/6).

- [ ] **Step 6: Commit**

```bash
git add tool/web/search_ddg.go tool/web/search_ddg_test.go go.mod go.sum
git commit -m "feat(tool/web): DuckDuckGo provider (goquery HTML scrape)"
```

---

## Task 9: Search Dispatcher

Implement the `searchDispatcher` on top of Task 4's cache. Handles provider registration, `resolveProvider` selection logic, and the `Handler()` factory that returns a `tool.Handler` consumable by `tool.Registry`.

**Files:**
- Modify: `tool/web/search_dispatcher.go` (append dispatcher to the cache file from Task 4)
- Modify: `tool/web/search_dispatcher_test.go` (append resolver + handler tests)

- [ ] **Step 1: Write the failing test**

Append to `tool/web/search_dispatcher_test.go`:

```go
// fakeProvider is a stub SearchProvider used to test dispatcher logic
// without hitting the network.
type fakeProvider struct {
	id         string
	configured bool
	results    []SearchResult
	err        error
	calls      int
}

func (f *fakeProvider) ID() string        { return f.id }
func (f *fakeProvider) Configured() bool  { return f.configured }
func (f *fakeProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func dispatcherWith(providers map[string]SearchProvider, explicit string) *searchDispatcher {
	return &searchDispatcher{
		providers: providers,
		explicit:  explicit,
		cache:     newSearchCache(8, time.Minute),
	}
}

func TestDispatcher_ExplicitProviderWins(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_API_KEY", "")
	t.Setenv("EXA_API_KEY", "")
	providers := map[string]SearchProvider{
		"tavily": &fakeProvider{id: "tavily", configured: true},
		"brave":  &fakeProvider{id: "brave", configured: true},
		"exa":    &fakeProvider{id: "exa", configured: true},
		"ddg":    &fakeProvider{id: "ddg", configured: true},
	}
	d := dispatcherWith(providers, "brave")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "brave", p.ID())
}

func TestDispatcher_ExplicitUnknownErrors(t *testing.T) {
	providers := map[string]SearchProvider{
		"ddg": &fakeProvider{id: "ddg", configured: true},
	}
	d := dispatcherWith(providers, "bogus")
	_, err := d.resolveProvider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestDispatcher_ExplicitUnconfiguredErrors(t *testing.T) {
	providers := map[string]SearchProvider{
		"tavily": &fakeProvider{id: "tavily", configured: false},
		"ddg":    &fakeProvider{id: "ddg", configured: true},
	}
	d := dispatcherWith(providers, "tavily")
	_, err := d.resolveProvider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestDispatcher_AutoPriority(t *testing.T) {
	// Only brave and exa configured; priority should pick brave.
	providers := map[string]SearchProvider{
		"tavily": &fakeProvider{id: "tavily", configured: false},
		"brave":  &fakeProvider{id: "brave", configured: true},
		"exa":    &fakeProvider{id: "exa", configured: true},
		"ddg":    &fakeProvider{id: "ddg", configured: true},
	}
	d := dispatcherWith(providers, "")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "brave", p.ID())
}

func TestDispatcher_AutoFallsBackToDDG(t *testing.T) {
	providers := map[string]SearchProvider{
		"tavily": &fakeProvider{id: "tavily", configured: false},
		"brave":  &fakeProvider{id: "brave", configured: false},
		"exa":    &fakeProvider{id: "exa", configured: false},
		"ddg":    &fakeProvider{id: "ddg", configured: true},
	}
	d := dispatcherWith(providers, "")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "ddg", p.ID())
}

func TestDispatcher_HandlerCachesRepeatedQueries(t *testing.T) {
	fake := &fakeProvider{
		id:         "ddg",
		configured: true,
		results:    []SearchResult{{Title: "T", URL: "https://x", Snippet: "S"}},
	}
	providers := map[string]SearchProvider{"ddg": fake}
	d := dispatcherWith(providers, "ddg")
	h := d.Handler()

	out1, err := h(context.Background(), json.RawMessage(`{"query":"golang"}`))
	require.NoError(t, err)
	out2, err := h(context.Background(), json.RawMessage(`{"query":"golang"}`))
	require.NoError(t, err)

	assert.Equal(t, out1, out2, "cached output must match")
	assert.Equal(t, 1, fake.calls, "second call should hit cache")
}

func TestDispatcher_HandlerDifferentQueriesBypassCache(t *testing.T) {
	fake := &fakeProvider{id: "ddg", configured: true, results: []SearchResult{{Title: "T"}}}
	providers := map[string]SearchProvider{"ddg": fake}
	d := dispatcherWith(providers, "ddg")
	h := d.Handler()

	_, _ = h(context.Background(), json.RawMessage(`{"query":"a"}`))
	_, _ = h(context.Background(), json.RawMessage(`{"query":"b"}`))

	assert.Equal(t, 2, fake.calls)
}

func TestDispatcher_HandlerSerializesResult(t *testing.T) {
	score := 0.7
	fake := &fakeProvider{
		id:         "tavily",
		configured: true,
		results: []SearchResult{
			{Title: "T1", URL: "https://x", Snippet: "S1", Score: &score},
			{Title: "T2", URL: "https://y", Snippet: "S2"},
		},
	}
	providers := map[string]SearchProvider{"tavily": fake}
	d := dispatcherWith(providers, "tavily")
	h := d.Handler()

	out, err := h(context.Background(), json.RawMessage(`{"query":"golang","num_results":2}`))
	require.NoError(t, err)

	var result struct {
		Query    string         `json:"query"`
		Provider string         `json:"provider"`
		Results  []SearchResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "golang", result.Query)
	assert.Equal(t, "tavily", result.Provider)
	require.Len(t, result.Results, 2)
	assert.Equal(t, "T1", result.Results[0].Title)
	require.NotNil(t, result.Results[0].Score)
	assert.Nil(t, result.Results[1].Score)
}

func TestDispatcher_HandlerRejectsEmptyQuery(t *testing.T) {
	d := dispatcherWith(map[string]SearchProvider{"ddg": &fakeProvider{id: "ddg", configured: true}}, "ddg")
	out, err := d.Handler()(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "query")
}

func TestDispatcher_HandlerWrapsProviderError(t *testing.T) {
	fake := &fakeProvider{id: "ddg", configured: true, err: errors.New("http 500")}
	d := dispatcherWith(map[string]SearchProvider{"ddg": fake}, "ddg")
	out, err := d.Handler()(context.Background(), json.RawMessage(`{"query":"q"}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "ddg")
	assert.Contains(t, out, "http 500")
}

func TestDispatcher_HandlerClampsNumResults(t *testing.T) {
	fake := &fakeProvider{id: "ddg", configured: true, results: []SearchResult{{Title: "T"}}}
	d := dispatcherWith(map[string]SearchProvider{"ddg": fake}, "ddg")

	// num_results = 0 → clamped to 5 (default)
	_, _ = d.Handler()(context.Background(), json.RawMessage(`{"query":"q"}`))
	// num_results = 30 → clamped to 20 (max)
	_, _ = d.Handler()(context.Background(), json.RawMessage(`{"query":"q2","num_results":30}`))
	// Both should have succeeded without panic.
	assert.Equal(t, 2, fake.calls)
}
```

Update the imports at the top of `tool/web/search_dispatcher_test.go` to include:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tool/web/ -run TestDispatcher -v`
Expected: FAIL with `searchDispatcher undefined` / `d.Handler undefined`.

- [ ] **Step 3: Write the dispatcher**

Append to `tool/web/search_dispatcher.go` (keep the cache code from Task 4 as-is at the top of the file; add below it):

```go
// searchDispatcher picks a SearchProvider based on configuration,
// checks the cache, and runs the chosen provider. It exposes a
// tool.Handler shape via Handler() so the tool.Registry can invoke it
// directly.
type searchDispatcher struct {
	providers map[string]SearchProvider
	explicit  string // opts.SearchProvider — "" means auto-priority
	cache     *searchCache
}

// priorityOrder is the auto-select sequence. DDG is always last
// because it is the keyless fallback.
var priorityOrder = []string{"tavily", "brave", "exa", "ddg"}

// newSearchDispatcher constructs a dispatcher from the caller's
// Options. DDG is always registered; the other three are registered
// regardless of key presence and Configured() reports the real state.
func newSearchDispatcher(opts Options) *searchDispatcher {
	return &searchDispatcher{
		providers: map[string]SearchProvider{
			"ddg":    newDDGProvider(""),
			"tavily": newTavilyProvider(opts.TavilyAPIKey, ""),
			"brave":  newBraveProvider(opts.BraveAPIKey, ""),
			"exa":    newExaProvider(opts.ExaAPIKey, ""),
		},
		explicit: opts.SearchProvider,
		cache:    newSearchCache(128, 60*time.Second),
	}
}

// resolveProvider picks the active SearchProvider.
func (d *searchDispatcher) resolveProvider() (SearchProvider, error) {
	if d.explicit != "" {
		p, ok := d.providers[d.explicit]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", d.explicit)
		}
		if !p.Configured() {
			return nil, fmt.Errorf("provider %q not configured", d.explicit)
		}
		return p, nil
	}
	for _, id := range priorityOrder {
		if p, ok := d.providers[id]; ok && p.Configured() {
			return p, nil
		}
	}
	// Unreachable: ddg is always Configured.
	return nil, fmt.Errorf("no search provider configured")
}
```

Also append the handler factory:

```go
// searchArgs is the dispatcher's input shape. Named distinct from the
// legacy `webSearchArgs` still living in search.go so both can coexist
// until Task 10 strips the legacy block.
type searchArgs struct {
	Query      string `json:"query"`
	NumResults int    `json:"num_results,omitempty"`
}

type webSearchPayload struct {
	Query    string         `json:"query"`
	Provider string         `json:"provider"`
	Results  []SearchResult `json:"results"`
}

// Handler returns a tool.Handler that runs the dispatcher pipeline:
// parse args → resolveProvider → cache.Get → provider.Search →
// normalize → cache.Set. All errors surface as tool.ToolError JSON
// so the LLM sees a structured payload; the outer error return is
// always nil (matches existing search.go convention).
func (d *searchDispatcher) Handler() tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args searchArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Query == "" {
			return tool.ToolError("query is required"), nil
		}
		n := args.NumResults
		if n <= 0 {
			n = 5
		}
		if n > 20 {
			n = 20
		}

		provider, err := d.resolveProvider()
		if err != nil {
			log.Printf("[web_search] resolve err=%v", err)
			return tool.ToolError(err.Error()), nil
		}

		cacheKey := fmt.Sprintf("%s|%s|%d", provider.ID(), strings.ToLower(args.Query), n)
		if cached, ok := d.cache.Get(cacheKey); ok {
			return tool.ToolResult(webSearchPayload{
				Query:    args.Query,
				Provider: provider.ID(),
				Results:  cached,
			}), nil
		}

		results, err := provider.Search(ctx, args.Query, n)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				// Context-driven errors skip the tool.ToolError wrap;
				// return the context error so the Engine stops the turn.
				return "", ctxErr
			}
			log.Printf("[web_search] provider=%s err=%v", provider.ID(), err)
			return tool.ToolError(provider.ID() + ": " + err.Error()), nil
		}

		d.cache.Set(cacheKey, results)
		return tool.ToolResult(webSearchPayload{
			Query:    args.Query,
			Provider: provider.ID(),
			Results:  results,
		}), nil
	}
}
```

Update the imports at the top of `tool/web/search_dispatcher.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/hermind/tool"
)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tool/web/ -run TestDispatcher -v`
Expected: PASS (11/11).

Run: `go test ./tool/web/ -v -race`
Expected: PASS (all previously-written tests too).

- [ ] **Step 5: Commit**

```bash
git add tool/web/search_dispatcher.go tool/web/search_dispatcher_test.go
git commit -m "feat(tool/web): searchDispatcher — resolve + cache + handler"
```

---

## Task 10: Wire RegisterAll + Strip Old Exa

Update `tool/web/register.go` to the new `Options` signature. Strip the inline Exa schema and handler from `tool/web/search.go`, keeping only the shared JSON schema plus a thin hand-off that uses the dispatcher.

**Files:**
- Modify: `tool/web/register.go`
- Modify: `tool/web/search.go`

- [ ] **Step 1: Write the failing test**

Update the imports at the top of `tool/web/search_dispatcher_test.go` to add the `tool` package:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Then append to the file:

```go
func TestRegisterAll_RegistersSearchWhenEnabled(t *testing.T) {
	t.Setenv("EXA_API_KEY", "")
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_API_KEY", "")
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{})
	// Use empty args — the dispatcher rejects with "query is required"
	// before any provider is invoked, so no live network call.
	out, err := reg.Dispatch(context.Background(), "web_search", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "query", "handler rejected empty query (tool IS registered)")
	assert.NotContains(t, out, "unknown tool")
}

func TestRegisterAll_RegistersFetchAlways(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{})
	out, err := reg.Dispatch(context.Background(), "web_fetch", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "url")
}

func TestRegisterAll_RegistersExtractWhenKeyPresent(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{FirecrawlAPIKey: "test"})
	out, err := reg.Dispatch(context.Background(), "web_extract", json.RawMessage(`{}`))
	require.NoError(t, err)
	// web_extract without a URL should return an error payload, which
	// proves the tool is registered (unregistered tools return
	// "unknown tool: ...").
	assert.Contains(t, out, `"error"`)
	assert.NotContains(t, out, "unknown tool")
}

func TestRegisterAll_SkipsExtractWithoutKey(t *testing.T) {
	t.Setenv("FIRECRAWL_API_KEY", "")
	reg := tool.NewRegistry()
	RegisterAll(reg, Options{})
	out, err := reg.Dispatch(context.Background(), "web_extract", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "unknown tool")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tool/web/ -run TestRegisterAll -v`
Expected: FAIL with `too many arguments in call to RegisterAll` or similar (old signature is `RegisterAll(reg, exaKey, firecrawlKey)`).

- [ ] **Step 3: Rewrite `tool/web/register.go`**

Replace the entire contents of `tool/web/register.go` with:

```go
// tool/web/register.go
package web

import (
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
)

// RegisterAll wires the web toolset into reg according to opts.
//
//   - web_fetch is always registered (uses stdlib http, no credentials).
//   - web_search is always registered; the dispatcher chooses a provider
//     based on opts.SearchProvider or built-in priority. DDG is the
//     keyless fallback so this tool is never unavailable.
//   - web_extract is registered only when opts.FirecrawlAPIKey is
//     non-empty (the existing behaviour is preserved).
func RegisterAll(reg *tool.Registry, opts Options) {
	reg.Register(&tool.Entry{
		Name:        "web_fetch",
		Toolset:     "web",
		Description: "Fetch a URL and return status + headers + body (max 2 MiB).",
		Emoji:       "🌐",
		Handler:     webFetchHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "web_fetch",
				Description: "Perform an HTTP GET/POST to a URL and return the response.",
				Parameters:  json.RawMessage(webFetchSchema),
			},
		},
	})

	dispatcher := newSearchDispatcher(opts)
	reg.Register(&tool.Entry{
		Name:        "web_search",
		Toolset:     "web",
		Description: "Search the web via a configured provider (DuckDuckGo, Tavily, Brave, or Exa).",
		Emoji:       "🔎",
		Handler:     dispatcher.Handler(),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "web_search",
				Description: "Search the web and return a list of results.",
				Parameters:  json.RawMessage(webSearchSchema),
			},
		},
	})

	if opts.FirecrawlAPIKey != "" {
		reg.Register(&tool.Entry{
			Name:        "web_extract",
			Toolset:     "web",
			Description: "Extract page content as markdown/html/text via Firecrawl.",
			Emoji:       "📰",
			Handler:     newWebExtractHandler(opts.FirecrawlAPIKey, ""),
			Schema: tool.ToolDefinition{
				Type: "function",
				Function: tool.FunctionDef{
					Name:        "web_extract",
					Description: "Extract the main content of a web page.",
					Parameters:  json.RawMessage(webExtractSchema),
				},
			},
		})
	}
}
```

- [ ] **Step 4: Trim `tool/web/search.go` to just the schema constant**

Replace the entire contents of `tool/web/search.go` with:

```go
// tool/web/search.go
package web

// webSearchSchema is the JSON Schema for web_search tool arguments.
// The actual handler lives in search_dispatcher.go — this file holds
// only the shared schema string so the dispatcher and any future
// callers share a single source of truth.
const webSearchSchema = `{
  "type": "object",
  "properties": {
    "query":       { "type": "string", "description": "Search query" },
    "num_results": { "type": "number", "description": "Number of results to return (default 5, max 20)" }
  },
  "required": ["query"]
}`
```

This removes the inline `exaSearchRequest`, `exaSearchResponse`, `exaResult`, `webSearchResult`, `webSearchArgs`, and `newWebSearchHandler` symbols from the file. All Exa-specific types now live in `search_exa.go` under the unexported names `exaRequest`, `exaResponse`, `exaResultItem`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./tool/web/ -v`
Expected: PASS (all — fetch, extract, provider tests, dispatcher tests, RegisterAll tests).

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add tool/web/register.go tool/web/search.go tool/web/search_dispatcher_test.go
git commit -m "feat(tool/web): new RegisterAll(opts) signature; strip legacy Exa"
```

---

## Task 11: Update Call Site in cli/repl.go

Rewire `cli/repl.go:128-130` to build and pass the new `web.Options` struct.

**Files:**
- Modify: `cli/repl.go`

- [ ] **Step 1: Read the current call site**

Run: `grep -n "web.RegisterAll\|EXA_API_KEY\|FIRECRAWL_API_KEY" cli/repl.go`
Expected:
```
128:	exaKey := os.Getenv("EXA_API_KEY")
129:	firecrawlKey := os.Getenv("FIRECRAWL_API_KEY")
130:	web.RegisterAll(toolRegistry, exaKey, firecrawlKey)
```

The config variable in this function is named `app.Config` (a `*config.Config`), not `cfg`. All other tool registrations in the file use `app.Config.<Section>` — follow the same convention.

- [ ] **Step 2: Replace the block**

Open `cli/repl.go` and replace the three lines:

```go
	exaKey := os.Getenv("EXA_API_KEY")
	firecrawlKey := os.Getenv("FIRECRAWL_API_KEY")
	web.RegisterAll(toolRegistry, exaKey, firecrawlKey)
```

with:

```go
	web.RegisterAll(toolRegistry, web.Options{
		SearchProvider:  app.Config.Web.Search.Provider,
		TavilyAPIKey:    app.Config.Web.Search.Providers.Tavily.APIKey,
		BraveAPIKey:     app.Config.Web.Search.Providers.Brave.APIKey,
		ExaAPIKey:       app.Config.Web.Search.Providers.Exa.APIKey,
		FirecrawlAPIKey: os.Getenv("FIRECRAWL_API_KEY"),
	})
```

Note on fallback: the provider's `Configured()` method reads the env var when the config field is empty, so a user with only `EXA_API_KEY` set (and no `app.Config.Web.Search.Providers.Exa.APIKey`) still gets Exa via auto-priority.

- [ ] **Step 3: Verify os is still imported**

The `os` import stays referenced by the `os.Getenv("FIRECRAWL_API_KEY")` call. Run:

```bash
grep -n "^\s*\"os\"" cli/repl.go
```
Expected: one match in the import block.

- [ ] **Step 4: Run build + tests**

Run: `go build ./cli/...`
Expected: PASS (compiles).

Run: `go test ./cli/...`
Expected: PASS (existing tests unaffected).

Run: `go test ./...`
Expected: PASS (no regression anywhere).

- [ ] **Step 5: Commit**

```bash
git add cli/repl.go
git commit -m "feat(cli): wire web.Options from cfg.Web.Search"
```

**Drift note**: If Phase 1 (`web-chat-backend`) landed first, the call site has moved to `cli/engine_deps.go::BuildEngineDeps`. In that case, perform the same substitution there instead. If Phase 3 (`tui-removal`) landed first, `cli/repl.go` no longer exists — apply the edit to `cli/engine_deps.go` only.

---

## Task 12: CHANGELOG + Smoke Doc

Document the change for operators and provide a manual verification flow.

**Files:**
- Modify: `CHANGELOG.md`
- Create: `docs/smoke/web-search.md`

- [ ] **Step 1: Read the current CHANGELOG to find the Unreleased section**

Run: `head -40 CHANGELOG.md`
Expected: an `## [Unreleased]` section at the top, or a similar "WIP" marker.

- [ ] **Step 2: Prepend the new entries**

Below the existing `## [Unreleased]` heading, insert:

```markdown
### Added

- `web_search` tool now supports DuckDuckGo (keyless), Tavily, and Brave
  Search in addition to Exa. Provider is chosen via the new
  `web.search.provider` config field or auto-selected by priority
  (Tavily → Brave → Exa → DuckDuckGo). DuckDuckGo is the keyless
  fallback and always available.
- New top-level `web:` config section with `search.provider` and
  `search.providers.{tavily,brave,exa}.api_key` fields. Environment
  variables `EXA_API_KEY` (existing), `TAVILY_API_KEY` (new), and
  `BRAVE_API_KEY` (new) continue to work as fallback when the config
  field is empty.
- 60s in-memory LRU cache (max 128 entries) for repeated queries,
  keyed by provider + lowercased query + num_results.
- New dependency: `github.com/PuerkitoBio/goquery` (DuckDuckGo HTML
  parsing, MIT licensed).
- Smoke verification guide: `docs/smoke/web-search.md`.
```

If no `## [Unreleased]` section exists, add one at the top with today's date header directly below.

- [ ] **Step 3: Create the smoke guide**

Create `docs/smoke/web-search.md`:

```markdown
# Web Search Smoke Verification

Manual verification flow for the multi-provider `web_search` tool.
Each scenario exercises a different provider + config interaction.

## Prerequisites

- A working `hermind` build: `go build -o bin/hermind ./cli`
- A config file at `~/.hermind/config.yaml` (or point `--config` to
  one). The minimum viable config is an empty file; defaults cover
  everything the smoke flow needs.
- Optional: API keys for the paid providers you want to exercise.

## 1. DuckDuckGo fallback (no keys configured)

```bash
unset EXA_API_KEY TAVILY_API_KEY BRAVE_API_KEY
hermind run --prompt "search the web for 'golang tutorials' using web_search and summarize the top 3"
```

Expected:
- Agent invokes `web_search` with a `golang tutorials` query.
- Response JSON payload has `"provider": "ddg"`.
- At least one result present (DDG rate-limiting can cause an empty
  result set — retry with a different query if so).

## 2. Explicit provider via env var (Tavily)

```bash
export TAVILY_API_KEY="<your key>"
hermind run --prompt "search the web for 'kubernetes networking' and list the top 3 with snippets"
```

Expected:
- `"provider": "tavily"` in the tool result.
- Results include non-empty `snippet` fields and a `score` float.

## 3. Explicit provider via config

Edit `~/.hermind/config.yaml`:

```yaml
web:
  search:
    provider: brave
    providers:
      brave:
        api_key: "<your Brave key>"
```

Run:
```bash
hermind run --prompt "find me news about SpaceX from the last week using web_search"
```

Expected:
- `"provider": "brave"` in the tool result.
- Results include URL + description but no `score` field.

## 4. Auto-priority when multiple keys present

Configure keys for Tavily + Exa (no `provider:` pin):

```yaml
web:
  search:
    providers:
      tavily:
        api_key: "<tav>"
      exa:
        api_key: "<exa>"
```

Run any search. Expected: `"provider": "tavily"` (tavily wins per
priority order Tavily > Brave > Exa > DuckDuckGo).

## 5. Cache hit on repeat query

Within 60 seconds, ask the agent twice:
```bash
hermind run --prompt "use web_search to find 'rust async book' — do this twice in a row"
```

Expected: both calls return identical results; on the second call the
tool returns instantly (no HTTP round trip). Verify in logs that only
one `[web_search] provider=<id>` line per unique query is emitted.

## 6. Explicit-but-missing provider

Set `provider: brave` in config without providing `brave.api_key`
and without `BRAVE_API_KEY`:

```yaml
web:
  search:
    provider: brave
```

Run a search. Expected: tool returns an error payload
`{"error":"provider \"brave\" not configured; ..."}` — the agent
sees it and either retries differently or reports it.
```

- [ ] **Step 4: Verify lint / markdown renders**

Run: `go test ./...`
Expected: PASS (ensure no test regressed).

- [ ] **Step 5: Commit**

```bash
git add CHANGELOG.md docs/smoke/web-search.md
git commit -m "docs(changelog,smoke): Web search multi-provider"
```

---

## Final Verification

After Task 12, run the full verification suite:

```bash
go test ./... -race
go vet ./...
go build ./...
```

All three should complete without errors. The repository is then ready
for merge.
