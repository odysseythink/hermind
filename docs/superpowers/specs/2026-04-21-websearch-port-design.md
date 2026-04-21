# Web Search Multi-Provider Port — Design Spec

**Date:** 2026-04-21
**Status:** Approved (pending user review of this file)
**Source context:** Port the multi-provider web search infrastructure from
`/Users/ranwei/workspace/go_work/openclaw-2026.4.15` (TypeScript) into this
Go codebase, adapting it to `tool/web/`.

---

## 1. Goals & Non-Goals

### Goals

- Upgrade `tool/web/search.go` from a single Exa-only implementation to a
  pluggable multi-provider structure.
- v1 supports four providers: **DuckDuckGo**, **Tavily**, **Brave Search**,
  **Exa**.
- Selection strategy: `config.tools.web.search.provider` explicit → else
  built-in priority (Tavily > Brave > Exa > DuckDuckGo) → DuckDuckGo is the
  keyless fallback and always available.
- Unified result schema returned to the LLM:
  `{query, provider, results:[{title, url, snippet, published_date?, score?}]}`.
- In-memory LRU cache (max 128 entries, 60s TTL), keyed by
  `{providerID, lowercase(query), num_results}`.
- Config schema cuts cleanly to the new layout; old `tools.web.exa_api_key`
  is removed with a CHANGELOG migration note and a startup warning if the old
  field is present.

### Non-Goals

- Citation redirect (external URLs not rewritten).
- Other providers beyond the four above (Perplexity, Moonshot, Google,
  Firecrawl-search, XAI, SearXNG, Exa `/contents` etc.) — leave extension
  points, do not implement.
- Runtime credential UI — config.yaml + env vars only.
- No change to `tool/web/fetch.go` or `tool/web/extract.go` (Firecrawl
  extract) behaviour.
- No multi-provider concurrent fallback — selected provider fails, we
  report the error. The LLM can retry.
- No cross-provider result dedup, no real upstream integration tests.

---

## 2. Architecture

```
┌────────────────────────────────────────────────────────────┐
│  RegisterAll(reg, opts Options)                            │
│    ↓                                                        │
│  newSearchDispatcher(opts.Search) ─► SearchProvider[4]     │
│                                                             │
│  dispatcher.Search(ctx, args):                              │
│    ├─► resolveProvider() → SearchProvider (has providerID) │
│    ├─► cache.Get(providerID|query|num) — if hit, return    │
│    ├─► provider.Search(ctx, query, num)                     │
│    ├─► normalizeResults → []SearchResult                    │
│    └─► cache.Set + return {query, provider, results}        │
└────────────────────────────────────────────────────────────┘
```

### Core interface

```go
type SearchProvider interface {
    ID() string           // "ddg" | "tavily" | "brave" | "exa"
    Configured() bool     // key present (or always true for ddg)
    Search(ctx context.Context, q string, n int) ([]SearchResult, error)
}

type SearchResult struct {
    Title         string    `json:"title"`
    URL           string    `json:"url"`
    Snippet       string    `json:"snippet"`
    PublishedDate string    `json:"published_date,omitempty"`
    Score         *float64  `json:"score,omitempty"`
}
```

`Score` is a pointer so providers that have no relevance score (DDG, Brave)
emit `omitempty` rather than `0.0`.

### Dispatcher

- `newSearchDispatcher(cfg SearchConfig)` — constructs 4 provider instances;
  providers with no key set `Configured()` to `false`, except DDG which is
  always `true`.
- `resolveProvider()` — explicit `cfg.Provider` wins; if unset, walks the
  priority list; returns error if explicit provider is unknown or not
  configured.
- `Search(ctx, args)` — runs resolve → cache.Get → provider.Search →
  normalize → cache.Set.

### Registration point

The current call site is `cli/repl.go:128-130`:

```go
exaKey := os.Getenv("EXA_API_KEY")
firecrawlKey := os.Getenv("FIRECRAWL_API_KEY")
web.RegisterAll(toolRegistry, exaKey, firecrawlKey)
```

This becomes:

```go
web.RegisterAll(toolRegistry, web.Options{
    Search:          cfg.Web.Search,                // new config struct
    FirecrawlAPIKey: os.Getenv("FIRECRAWL_API_KEY"), // unchanged env-var path
})
```

**Sequencing with pending TUI→Web plans**: the Phase 1 plan (`web-chat-backend`)
extracts this block into `cli/engine_deps.go::BuildEngineDeps`, and the Phase 3
plan (`tui-removal`) deletes `cli/repl.go`. Whichever lands first — the
websearch plan targets the file *as it currently exists*; execution will
rebase onto whatever state is on `main` at the time.

---

## 3. File Layout

### New files

```
tool/web/search_provider.go       SearchProvider interface, SearchResult, helpers
tool/web/search_dispatcher.go     searchDispatcher, cache, resolveProvider
tool/web/search_ddg.go            DuckDuckGo HTML impl
tool/web/search_tavily.go         Tavily API impl
tool/web/search_brave.go          Brave Search API impl
tool/web/search_exa.go            Exa API impl (from current search.go)
```

### Modified files

```
tool/web/search.go                keep file; strip Exa logic, keep
                                  webSearchSchema + buildWebSearchHandler
tool/web/register.go              RegisterAll(reg, opts Options)
tool/web/web_test.go              split: keep fetch/extract tests here,
                                  move Exa to search_exa_test.go
cli/repl.go                       call new RegisterAll signature (or
                                  cli/engine_deps.go if Phase 1 has
                                  landed first)
config/config.go                  add top-level Web WebConfig field,
                                  plus SearchConfig + SearchProvidersConfig
                                  + ProviderKeyConfig struct definitions
config/descriptor/web.go          NEW: descriptor for web.search.*
CHANGELOG.md                      additive entry: new providers + cache
```

### New test files

```
tool/web/search_dispatcher_test.go   selection priority, cache hit/expiry/LRU
tool/web/search_ddg_test.go          httptest HTML fixture + CAPTCHA fixture
tool/web/search_tavily_test.go       httptest JSON fixture
tool/web/search_brave_test.go        httptest JSON fixture + header check
tool/web/search_exa_test.go          httptest JSON fixture (migrated)
```

### Unchanged

```
tool/web/fetch.go
tool/web/extract.go
```

---

## 4. Provider Details

### 4.1 DuckDuckGo (`search_ddg.go`)

- **Endpoint**: `POST https://html.duckduckgo.com/html/`
- **Auth**: none
- **Request**: `application/x-www-form-urlencoded`, body `q=<query>`
- **Response**: HTML, parsed with `github.com/PuerkitoBio/goquery`:
  - title ← `.result__a` text
  - url ← `.result__a[href]`, extract `uddg` query param then
    `url.QueryUnescape` it (DDG wraps outbound links as
    `/l/?uddg=<base64>`)
  - snippet ← `.result__snippet` text
  - published_date / score: none
- **Errors**:
  - HTML body contains `anomaly` → `tool.ToolError("ddg rate limited")`
  - Non-200 → `tool.ToolError("ddg http <code>")`
- `Configured()` always returns `true`.

### 4.2 Tavily (`search_tavily.go`)

- **Endpoint**: `POST https://api.tavily.com/search`
- **Auth**: `api_key` field in request body
- **Request**:
  ```json
  {"api_key":"...","query":"...","max_results":5,"include_answer":false,"include_raw_content":false}
  ```
- **Response**:
  ```json
  {"query":"...","results":[{"title":"...","url":"...","content":"...","score":0.8,"published_date":"..."}]}
  ```
- **Field mapping**: title→title, url→url, content→snippet,
  published_date→published_date (raw string), score→score.
- **Env var**: `TAVILY_API_KEY`.

### 4.3 Brave Search (`search_brave.go`)

- **Endpoint**: `GET https://api.search.brave.com/res/v1/web/search?q=<query>&count=<n>`
- **Auth**: headers `X-Subscription-Token: <api_key>` + `Accept: application/json`
- **Response**:
  ```json
  {"web":{"results":[{"title":"...","url":"...","description":"...","age":"...","page_age":"2024-..."}]}}
  ```
- **Field mapping**: title→title, url→url, description→snippet,
  `page_age`→published_date (empty if absent; `age` field is a relative
  string, unused). No score.
- **Env var**: `BRAVE_API_KEY`.

### 4.4 Exa (`search_exa.go`)

- **Endpoint**: `POST https://api.exa.ai/search`
- **Auth**: header `x-api-key: <api_key>`
- **Request**: `{"query":"...", "numResults":5}` (matches current
  `search.go` behaviour — no `contents` field)
- **Response**:
  ```json
  {"results":[{"title":"...","url":"...","text":"","publishedDate":"...","score":0.7}]}
  ```
- **Field mapping**: title→title, url→url, text→snippet (may be empty when
  `/search` basic endpoint is used — acceptable), publishedDate→
  published_date, score→score.
- **Env var**: `EXA_API_KEY`.

### 4.5 Shared HTTP defaults

- `http.Client{Timeout: 30 * time.Second}` — one client per provider,
  created in the ctor.
- Non-2xx → `tool.ToolError("<providerID> http <code>")`.
- Decode error → `tool.ToolError("<providerID> decode: <msg>")`.
- Context cancel/timeout propagates; wrapped as
  `tool.ToolError("<providerID> timeout")` only if `ctx.Err() == nil` at
  the moment of failure (otherwise the Engine layer sees the context
  error verbatim).

### 4.6 New dependency

- `github.com/PuerkitoBio/goquery` — only DDG uses it; MIT licensed,
  stable, ~300 KB. Added to `go.mod`; noted in CHANGELOG's Added section.

---

## 5. Selection & Config Schema

### 5.1 resolveProvider

```go
func (d *searchDispatcher) resolveProvider() (SearchProvider, error) {
    if d.cfg.Provider != "" {
        p, ok := d.providers[d.cfg.Provider]
        if !ok {
            return nil, fmt.Errorf("unknown provider %q", d.cfg.Provider)
        }
        if !p.Configured() {
            return nil, fmt.Errorf("provider %q not configured", d.cfg.Provider)
        }
        return p, nil
    }
    for _, id := range []string{"tavily", "brave", "exa", "ddg"} {
        if p := d.providers[id]; p.Configured() {
            return p, nil
        }
    }
    // unreachable: ddg is always Configured
    return nil, errors.New("no search provider configured")
}
```

### 5.2 New config YAML

New top-level `web:` section (matches existing convention where each tool
family — `browser:`, `memory:`, `terminal:` — lives at the top level):

```yaml
web:
  search:
    provider: tavily              # optional; empty → auto priority
    providers:
      tavily:
        api_key: "..."
      brave:
        api_key: "..."
      exa:
        api_key: "..."
      # ddg has no sub-node; always enabled
```

Firecrawl (used by `tool/web/extract.go`) continues to read
`FIRECRAWL_API_KEY` env var directly — out of scope for this spec.

### 5.3 Go config types

New types added to `config/config.go`, using `yaml` tags to match the
existing convention (no `mapstructure`):

```go
// config.Config gains:
//   Web WebConfig `yaml:"web,omitempty"`

type WebConfig struct {
    Search SearchConfig `yaml:"search,omitempty"`
}

type SearchConfig struct {
    Provider  string                `yaml:"provider,omitempty"`
    Providers SearchProvidersConfig `yaml:"providers,omitempty"`
}

type SearchProvidersConfig struct {
    Tavily ProviderKeyConfig `yaml:"tavily,omitempty"`
    Brave  ProviderKeyConfig `yaml:"brave,omitempty"`
    Exa    ProviderKeyConfig `yaml:"exa,omitempty"`
}

type ProviderKeyConfig struct {
    APIKey string `yaml:"api_key,omitempty"`
}
```

### 5.4 Env var fallback

Each provider's ctor resolves key as `cfg.APIKey` first, then
`os.Getenv(envName)`:

| Provider | Env var         |
|----------|-----------------|
| tavily   | `TAVILY_API_KEY`|
| brave    | `BRAVE_API_KEY` |
| exa      | `EXA_API_KEY`   |
| ddg      | (none)          |

### 5.5 Descriptor

NEW file `config/descriptor/web.go` (matches the one-file-per-section
pattern of `browser.go`, `memory.go`, etc.) publishes:

- `web.search.provider` — enum `["", "tavily", "brave", "exa", "ddg"]`
- `web.search.providers.tavily.api_key` — secret
- `web.search.providers.brave.api_key` — secret
- `web.search.providers.exa.api_key` — secret

DDG has no UI field. Firecrawl stays out.

---

## 6. Cache

```go
type searchCache struct {
    mu      sync.Mutex
    entries map[string]cacheEntry
    order   []string          // LRU head, MRU tail
    maxSize int               // 128
    ttl     time.Duration     // 60 * time.Second
}

type cacheEntry struct {
    value     []SearchResult
    provider  string
    expiresAt time.Time
}
```

- **Key**: `"{providerID}|{lowercase(query)}|{num}"` (after num is
  clamped to 1..20).
- **Get**: miss if absent or expired (delete expired on access). Hit
  moves key to MRU.
- **Set**: evict LRU entry if at capacity, then insert at MRU tail.
- **Concurrency**: one `sync.Mutex` around all ops; contention is not a
  concern at 128 entries.
- **Per-provider isolation**: provider switches don't leak cross-provider
  stale data.

No third-party LRU library; `order` slice + linear search is fine at
this scale.

---

## 7. Error Handling

### Responsibility split

- **`SearchProvider.Search`** returns Go `error`. Providers do not build
  `tool.ToolError` strings themselves — they just return descriptive errors
  like `fmt.Errorf("http %d", code)` or `fmt.Errorf("decode: %w", err)`.
- **`searchDispatcher.Search`** is the tool handler shape. It catches
  provider errors and converts them into `tool.ToolError("<providerID>: <msg>")`
  before returning to the Engine. Argument-level errors (bad JSON, empty
  query) are handled before reaching a provider.

### Error categories and surface

1. **Argument errors** (empty query, bad JSON args from the model) —
   dispatcher returns `tool.ToolError(...)`, no provider call.
2. **Provider not configured** (`cfg.Provider` explicit but key missing) —
   `resolveProvider` returns an error; dispatcher wraps as
   `tool.ToolError("provider \"<id>\" not configured; set <ENVVAR> or configure tools.web.search.providers.<id>.api_key")`.
3. **Non-2xx HTTP** — provider returns `fmt.Errorf("http %d", code)`;
   dispatcher wraps as `tool.ToolError("<providerID>: http <code>")`.
4. **JSON decode / HTML parse** — provider returns
   `fmt.Errorf("decode: %w", err)`; dispatcher wraps as
   `tool.ToolError("<providerID>: decode: <msg>")`.
5. **Context cancel/timeout** — dispatcher checks `ctx.Err()`; if context
   is cancelled/timed-out, return the context error verbatim (Engine layer
   handles it); otherwise wrap as `tool.ToolError("<providerID>: timeout")`.
6. **DDG CAPTCHA** — provider returns `errors.New("rate limited")` when
   HTML body contains `anomaly`; dispatcher wraps as
   `tool.ToolError("ddg: rate limited")`.

### Not done

- No auto-fallback to another provider on failure.
- No retry on transient failure.
- Errors not cached (only successful results are cached).

### Log

Dispatcher logs `[web_search] provider=<id> err=<msg>` before returning
the tool error. Success path logs nothing.

---

## 8. Testing

### Unit tests

| File | Coverage |
|------|----------|
| `search_dispatcher_test.go` | selection priority; explicit provider missing key → error; cache hit / TTL expiry / LRU eviction; concurrent access |
| `search_ddg_test.go` | HTML fixture → 3 results; uddg URL decode; CAPTCHA fixture → error |
| `search_tavily_test.go` | JSON fixture → field mapping; auth in body |
| `search_brave_test.go` | JSON fixture → field mapping; `X-Subscription-Token` header asserted |
| `search_exa_test.go` | JSON fixture → field mapping; migrated from current `web_test.go` |

### Test infrastructure

- `httptest.NewServer` per test; provider ctors accept an `endpoint`
  override param (mirrors current `newWebSearchHandler` signature).
- No gomock, no testify; project standard.
- Fixtures inline as constants in each test file (small HTML/JSON
  snippets).

### What is **not** tested

- Real upstream API calls (key handling + CI risk).
- HTML parser against live DDG variations (fixture only).

---

## 9. Migration & Compatibility

### No end-user breaking change

The existing setup is env-var-only (`EXA_API_KEY` → Exa `web_search`).
That keeps working unchanged. Everything in this spec is additive:

- New top-level `web:` config section (optional).
- New `TAVILY_API_KEY` / `BRAVE_API_KEY` env vars (optional).
- New providers, selection logic, cache.

Users who currently set only `EXA_API_KEY` continue to get Exa as the
search backend (since auto-priority picks Exa when it's the only
configured provider).

### Internal API change

`web.RegisterAll(reg, exaAPIKey, firecrawlAPIKey)` → `web.RegisterAll(reg, opts Options)`.
Internal to the repo, no external consumers.

### CHANGELOG.md new entry

```markdown
## [Unreleased]

### Added

- `web_search` tool now supports DuckDuckGo (keyless), Tavily, and Brave
  Search in addition to Exa. Provider is chosen via the new
  `web.search.provider` config field or auto-selected by priority
  (Tavily → Brave → Exa → DuckDuckGo).
- New `web` top-level config section; see `docs/smoke/web-search.md`.
- 60s in-memory LRU cache (max 128 entries) for repeated queries.
- New dependency: `github.com/PuerkitoBio/goquery` (DuckDuckGo HTML
  parsing, ~300 KB, MIT licensed).
- New env vars: `TAVILY_API_KEY`, `BRAVE_API_KEY`. `EXA_API_KEY` still
  works unchanged.
```

### Web frontend

`ConfigSection` already supports dotted field paths (commits `eafb67e`,
`48611b3`, `627adfe`). After adding `config/descriptor/web.go`, the new
fields render automatically.

---

## 10. Scope Boundary

### In

- 4 provider impls + dispatcher + cache.
- New config schema + descriptor update.
- Call-site change in `cli/engine_deps.go`.
- Per-provider tests + dispatcher tests.
- CHANGELOG entry.
- Smoke doc: `docs/smoke/web-search.md`.

### Out

- Citation redirect.
- Other providers (Perplexity, Moonshot, Google, Firecrawl-search,
  XAI, SearXNG, Exa `/contents`).
- Runtime UI for provider switching.
- Auto-fallback / retry.
- Cross-provider dedup.
- Live upstream integration tests.
- Any change to `fetch.go` / `extract.go`.

### Rough scale

- ~6 new Go files, ~900 lines impl + 500 lines tests.
- One implementation plan (≤ 18 tasks). No sub-project split needed.

---

## 11. Approval Checklist

- [x] Providers: DDG / Tavily / Brave / Exa
- [x] Selection: explicit config → priority Tavily > Brave > Exa > DDG
- [x] Result schema: `{query, provider, results:[{title, url, snippet, published_date?, score?}]}`
- [x] Cache: 60s LRU 128 entries
- [x] Backcompat: purely additive for end users (`EXA_API_KEY` still works); internal `RegisterAll` signature changes
- [x] Architecture: flat `tool/web/search_*.go`
- [x] Config shape: top-level `web:` section matching existing `browser:`/`memory:` convention
- [x] New dep: `goquery` (DDG only)
