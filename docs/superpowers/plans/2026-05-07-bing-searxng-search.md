# Bing & SearXNG 搜索供应商 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Bing HTML crawler and SearXNG self-hosted search providers to the `web_search` tool, with zero-API-key setup and mainland China accessibility.

**Architecture:** Two new `SearchProvider` implementations (`bingProvider`, `searxngProvider`) follow the exact pattern of existing Tavily/Brave/Exa/DDG providers. The dispatcher auto-priority order is extended to place SearXNG (when configured) and Bing (always available) between Exa and DuckDuckGo. Configuration flows through `config/config.go` → `config/descriptor/web.go` → `cli/engine_deps.go` → `tool/web/register.go`.

**Tech Stack:** Go, goquery (HTML parsing), httptest (testing), SearXNG JSON API

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `config/config.go` | Modify | Add `BingConfig`, `SearXNGConfig`, extend `SearchProvidersConfig` |
| `config/descriptor/web.go` | Modify | Add `"bing"`, `"searxng"` to provider enum; add UI fields for market and base_url |
| `tool/web/register.go` | Modify | Extend `Options` with `BingMarket` and `SearXNGBaseURL`; update `web_search` description |
| `tool/web/search_bing.go` | Create | Bing HTML crawler provider implementation |
| `tool/web/search_bing_test.go` | Create | Unit tests for Bing crawler (happy path, empty results, CAPTCHA, HTTP error) |
| `tool/web/search_searxng.go` | Create | SearXNG JSON API provider implementation |
| `tool/web/search_searxng_test.go` | Create | Unit tests for SearXNG (happy path, empty results, connection error) |
| `tool/web/search_dispatcher.go` | Modify | Register new providers; update `priorityOrder` |
| `tool/web/search_dispatcher_test.go` | Modify | Update auto-priority tests to include bing/searxng |
| `cli/engine_deps.go` | Modify | Wire new config fields into `web.Options` |

---

## Task 1: Update Config Structures

**Files:**
- Modify: `config/config.go`

- [ ] **Step 1: Add `BingConfig` and `SearXNGConfig` structs**

Insert these two new struct definitions immediately after `ProviderKeyConfig` (or near the other config types):

```go
type BingConfig struct {
	Market string `yaml:"market,omitempty"`
}

type SearXNGConfig struct {
	BaseURL string `yaml:"base_url,omitempty"`
}
```

- [ ] **Step 2: Extend `SearchProvidersConfig`**

Find `type SearchProvidersConfig struct` and add the two new fields:

```go
type SearchProvidersConfig struct {
	Tavily     ProviderKeyConfig `yaml:"tavily,omitempty"`
	Brave      ProviderKeyConfig `yaml:"brave,omitempty"`
	Exa        ProviderKeyConfig `yaml:"exa,omitempty"`
	DuckDuckGo *DDGProxyConfig   `yaml:"duckduckgo,omitempty"`
	Bing       BingConfig        `yaml:"bing,omitempty"`
	SearXNG    SearXNGConfig     `yaml:"searxng,omitempty"`
}
```

- [ ] **Step 3: Commit**

```bash
git add config/config.go
git commit -m "config: add Bing and SearXNG search provider configs"
```

---

## Task 2: Update UI Descriptor

**Files:**
- Modify: `config/descriptor/web.go`

- [ ] **Step 1: Update provider enum and help text**

Find the `search.provider` field and update it:

```go
{
	Name:  "search.provider",
	Label: "Search provider",
	Help:  "Leave blank to auto-select by priority (Tavily > Brave > Exa > SearXNG > Bing > DuckDuckGo).",
	Kind:  FieldEnum,
	Enum:  []string{"tavily", "brave", "exa", "searxng", "bing", "DuckDuckGo"},
},
```

- [ ] **Step 2: Add Bing market field**

Insert after the `exa` api_key field:

```go
{
	Name:        "search.providers.bing.market",
	Label:       "Bing market",
	Kind:        FieldString,
	Help:        "Market code for Bing results, e.g. zh-CN, en-US. Leave blank for default.",
	VisibleWhen: gate("bing"),
},
```

- [ ] **Step 3: Add SearXNG base_url field**

Insert after the Bing market field:

```go
{
	Name:        "search.providers.searxng.base_url",
	Label:       "SearXNG base URL",
	Kind:        FieldString,
	Help:        "Base URL of your SearXNG instance, e.g. http://localhost:8080.",
	VisibleWhen: gate("searxng"),
},
```

- [ ] **Step 4: Commit**

```bash
git add config/descriptor/web.go
git commit -m "descriptor: add Bing and SearXNG web search UI fields"
```

---

## Task 3: Update web.Options and Register

**Files:**
- Modify: `tool/web/register.go`

- [ ] **Step 1: Extend `Options` struct**

Find the `Options` struct and append two fields:

```go
type Options struct {
	SearchProvider  string
	TavilyAPIKey    string
	BraveAPIKey     string
	ExaAPIKey       string
	DDGProxyConfig  *config.DDGProxyConfig
	FirecrawlAPIKey string
	BingMarket      string
	SearXNGBaseURL  string
}
```

- [ ] **Step 2: Update `web_search` description**

Find the `web_search` registration block and update its `Description` to mention the new providers:

```go
Description: "Search the web via a configured provider (Tavily, Brave, Exa, SearXNG, Bing, or DuckDuckGo).",
```

- [ ] **Step 3: Commit**

```bash
git add tool/web/register.go
git commit -m "web: extend Options with BingMarket and SearXNGBaseURL"
```

---

## Task 4: Implement Bing Search Provider

**Files:**
- Create: `tool/web/search_bing.go`
- Create: `tool/web/search_bing_test.go`

- [ ] **Step 1: Write the failing test**

Create `tool/web/search_bing_test.go`:

```go
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBingProvider_HappyPath(t *testing.T) {
	var capturedQuery, capturedCount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Mozilla/5.0", r.Header.Get("User-Agent"))
		capturedQuery = r.URL.Query().Get("q")
		capturedCount = r.URL.Query().Get("count")

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<body>
<ol id="b_results">
<li class="b_algo">
<h2><a href="https://go.dev">The Go Programming Language</a></h2>
<div class="b_caption"><p>Go is an open source programming language.</p></div>
</li>
<li class="b_algo">
<h2><a href="https://go.dev/tour">A Tour of Go</a></h2>
<div class="b_caption"><p>Welcome to a tour of the Go programming language.</p></div>
</li>
</ol>
</body>
</html>`))
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	results, err := p.Search(context.Background(), "golang", 3)
	require.NoError(t, err)
	assert.Equal(t, "golang", capturedQuery)
	assert.Equal(t, "3", capturedCount)
	require.Len(t, results, 2)
	assert.Equal(t, "The Go Programming Language", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Equal(t, "Go is an open source programming language.", results[0].Snippet)
	assert.Equal(t, "A Tour of Go", results[1].Title)
}

func TestBingProvider_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><ol id="b_results"></ol></body></html>`))
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	results, err := p.Search(context.Background(), "xyz nonsense", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBingProvider_CAPTCHAError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div id="captcha">Please solve this CAPTCHA</div></body></html>`))
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "captcha")
}

func TestBingProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := newBingProvider("", srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 503")
}

func TestBingProvider_Configured(t *testing.T) {
	assert.True(t, newBingProvider("", "").Configured())
}

func TestBingProvider_ID(t *testing.T) {
	assert.Equal(t, "bing", newBingProvider("", "").ID())
}

func TestBingProvider_MarketParam(t *testing.T) {
	var capturedMarket string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMarket = r.URL.Query().Get("setmkt")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><ol id="b_results"></ol></body></html>`))
	}))
	defer srv.Close()

	p := newBingProvider("zh-CN", srv.URL)
	_, _ = p.Search(context.Background(), "q", 5)
	assert.Equal(t, "zh-CN", capturedMarket)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind && go test ./tool/web -run TestBingProvider -v
```

Expected: FAIL with `newBingProvider` undefined.

- [ ] **Step 3: Implement `search_bing.go`**

Create `tool/web/search_bing.go`:

```go
package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const bingDefaultURL = "https://www.bing.com/search"

type bingProvider struct {
	client   *http.Client
	endpoint string
	market   string
}

func newBingProvider(market, endpoint string) *bingProvider {
	return &bingProvider{
		client:   &http.Client{Timeout: httpTimeout},
		endpoint: endpoint,
		market:   market,
	}
}

func (p *bingProvider) ID() string      { return "bing" }
func (p *bingProvider) Configured() bool { return true }

func (p *bingProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = bingDefaultURL
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("bing: invalid endpoint: %w", err)
	}
	qval := u.Query()
	qval.Set("q", q)
	qval.Set("count", fmt.Sprintf("%d", n))
	if p.market != "" {
		qval.Set("setmkt", p.market)
	}
	u.RawQuery = qval.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bing: request error: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bing: http %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bing: parse error: %w", err)
	}

	bodyText := doc.Text()
	if strings.Contains(strings.ToLower(bodyText), "captcha") {
		return nil, fmt.Errorf("bing: captcha challenge detected")
	}

	var results []SearchResult
	doc.Find("li.b_algo").Each(func(i int, s *goquery.Selection) {
		if len(results) >= n {
			return
		}
		link := s.Find("h2 a")
		title := strings.TrimSpace(link.Text())
		href, _ := link.Attr("href")
		href = strings.TrimSpace(href)
		snippet := strings.TrimSpace(s.Find(".b_caption p").Text())
		if snippet == "" {
			snippet = strings.TrimSpace(s.Find("p").Text())
		}

		if title != "" && href != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     href,
				Snippet: snippet,
			})
		}
	})

	return results, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind && go test ./tool/web -run TestBingProvider -v
```

Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
git add tool/web/search_bing.go tool/web/search_bing_test.go
git commit -m "feat: add Bing HTML crawler search provider"
```

---

## Task 5: Implement SearXNG Search Provider

**Files:**
- Create: `tool/web/search_searxng.go`
- Create: `tool/web/search_searxng_test.go`

- [ ] **Step 1: Write the failing test**

Create `tool/web/search_searxng_test.go`:

```go
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

func TestSearXNGProvider_HappyPath(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		assert.Equal(t, "json", r.URL.Query().Get("format"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query": "golang",
			"results": []map[string]any{
				{"title": "Go", "url": "https://go.dev", "content": "Programming language", "publishedDate": "2024-03-01"},
				{"title": "Docs", "url": "https://go.dev/doc", "content": "Documentation"},
			},
		})
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL)
	results, err := p.Search(context.Background(), "golang", 7)
	require.NoError(t, err)
	assert.Equal(t, "golang", capturedQuery)
	require.Len(t, results, 2)
	assert.Equal(t, "Go", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Equal(t, "Programming language", results[0].Snippet)
	assert.Equal(t, "2024-03-01", results[0].PublishedDate)
	assert.Equal(t, "Docs", results[1].Title)
	assert.Equal(t, "", results[1].PublishedDate)
}

func TestSearXNGProvider_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL)
	results, err := p.Search(context.Background(), "xyz nonsense", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearXNGProvider_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newSearXNGProvider(srv.URL)
	_, err := p.Search(context.Background(), "q", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 500")
}

func TestSearXNGProvider_Configured(t *testing.T) {
	assert.False(t, newSearXNGProvider("").Configured())
	assert.True(t, newSearXNGProvider("http://localhost:8080").Configured())
}

func TestSearXNGProvider_ID(t *testing.T) {
	assert.Equal(t, "searxng", newSearXNGProvider("").ID())
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind && go test ./tool/web -run TestSearXNGProvider -v
```

Expected: FAIL with `newSearXNGProvider` undefined.

- [ ] **Step 3: Implement `search_searxng.go`**

Create `tool/web/search_searxng.go`:

```go
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type searxngProvider struct {
	baseURL string
	client  *http.Client
}

func newSearXNGProvider(baseURL string) *searxngProvider {
	return &searxngProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: httpTimeout},
	}
}

func (p *searxngProvider) ID() string      { return "searxng" }
func (p *searxngProvider) Configured() bool { return p.baseURL != "" }

func (p *searxngProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("searxng: base URL not configured")
	}

	u, err := url.Parse(p.baseURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("searxng: invalid base URL: %w", err)
	}
	qval := u.Query()
	qval.Set("q", q)
	qval.Set("format", "json")
	u.RawQuery = qval.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("searxng: request error: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng: http %d", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			URL           string `json:"url"`
			Title         string `json:"title"`
			Content       string `json:"content"`
			PublishedDate string `json:"publishedDate"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("searxng: decode error: %w", err)
	}

	var results []SearchResult
	for i, r := range payload.Results {
		if i >= n {
			break
		}
		results = append(results, SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Content,
			PublishedDate: r.PublishedDate,
		})
	}

	return results, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind && go test ./tool/web -run TestSearXNGProvider -v
```

Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
git add tool/web/search_searxng.go tool/web/search_searxng_test.go
git commit -m "feat: add SearXNG search provider"
```

---

## Task 6: Update Search Dispatcher

**Files:**
- Modify: `tool/web/search_dispatcher.go`

- [ ] **Step 1: Update `priorityOrder`**

Change:
```go
var priorityOrder = []string{"tavily", "brave", "exa", "DuckDuckGo"}
```

To:
```go
var priorityOrder = []string{"tavily", "brave", "exa", "searxng", "bing", "DuckDuckGo"}
```

- [ ] **Step 2: Register new providers in `newSearchDispatcher`**

Find the `providers` map in `newSearchDispatcher` and add the two new entries:

```go
func newSearchDispatcher(opts Options) *searchDispatcher {
	return &searchDispatcher{
		providers: map[string]SearchProvider{
			"DuckDuckGo": newDDGProvider(opts.DDGProxyConfig, ""),
			"tavily":     newTavilyProvider(opts.TavilyAPIKey, ""),
			"brave":      newBraveProvider(opts.BraveAPIKey, ""),
			"exa":        newExaProvider(opts.ExaAPIKey, ""),
			"bing":       newBingProvider(opts.BingMarket, ""),
			"searxng":    newSearXNGProvider(opts.SearXNGBaseURL),
		},
		explicit: opts.SearchProvider,
		cache:    newSearchCache(128, 60*time.Second),
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add tool/web/search_dispatcher.go
git commit -m "dispatcher: register Bing and SearXNG, update priority order"
```

---

## Task 7: Update Dispatcher Tests

**Files:**
- Modify: `tool/web/search_dispatcher_test.go`

- [ ] **Step 1: Update `TestDispatcher_AutoPriority`**

Replace the existing function body with one that includes `searxng` and `bing`:

```go
func TestDispatcher_AutoPriority(t *testing.T) {
	providers := map[string]SearchProvider{
		"tavily":     &fakeProvider{id: "tavily", configured: false},
		"brave":      &fakeProvider{id: "brave", configured: false},
		"exa":        &fakeProvider{id: "exa", configured: false},
		"searxng":    &fakeProvider{id: "searxng", configured: true},
		"bing":       &fakeProvider{id: "bing", configured: true},
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "searxng", p.ID())
}
```

- [ ] **Step 2: Update `TestDispatcher_AutoFallsBackToDDG`**

Replace the existing function body:

```go
func TestDispatcher_AutoFallsBackToDDG(t *testing.T) {
	providers := map[string]SearchProvider{
		"tavily":     &fakeProvider{id: "tavily", configured: false},
		"brave":      &fakeProvider{id: "brave", configured: false},
		"exa":        &fakeProvider{id: "exa", configured: false},
		"searxng":    &fakeProvider{id: "searxng", configured: false},
		"bing":       &fakeProvider{id: "bing", configured: false},
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "DuckDuckGo", p.ID())
}
```

- [ ] **Step 3: Update `TestDispatcher_ExplicitProviderWins`**

Replace the existing function body:

```go
func TestDispatcher_ExplicitProviderWins(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_API_KEY", "")
	t.Setenv("EXA_API_KEY", "")
	providers := map[string]SearchProvider{
		"tavily":     &fakeProvider{id: "tavily", configured: true},
		"brave":      &fakeProvider{id: "brave", configured: true},
		"exa":        &fakeProvider{id: "exa", configured: true},
		"searxng":    &fakeProvider{id: "searxng", configured: true},
		"bing":       &fakeProvider{id: "bing", configured: true},
		"DuckDuckGo": &fakeProvider{id: "DuckDuckGo", configured: true},
	}
	d := dispatcherWith(providers, "brave")
	p, err := d.resolveProvider()
	require.NoError(t, err)
	assert.Equal(t, "brave", p.ID())
}
```

- [ ] **Step 4: Run dispatcher tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind && go test ./tool/web -run TestDispatcher -v
```

Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
git add tool/web/search_dispatcher_test.go
git commit -m "test: update dispatcher tests for Bing and SearXNG priority"
```

---

## Task 8: Update Runtime Wiring

**Files:**
- Modify: `cli/engine_deps.go`

- [ ] **Step 1: Wire new config fields**

Find the `web.RegisterAll` call and add the two new fields:

```go
web.RegisterAll(toolRegistry, web.Options{
	SearchProvider:  app.Config.Web.Search.Provider,
	TavilyAPIKey:    app.Config.Web.Search.Providers.Tavily.APIKey,
	BraveAPIKey:     app.Config.Web.Search.Providers.Brave.APIKey,
	ExaAPIKey:       app.Config.Web.Search.Providers.Exa.APIKey,
	DDGProxyConfig:  app.Config.Web.Search.Providers.DuckDuckGo,
	FirecrawlAPIKey: os.Getenv("FIRECRAWL_API_KEY"),
	BingMarket:      app.Config.Web.Search.Providers.Bing.Market,
	SearXNGBaseURL:  app.Config.Web.Search.Providers.SearXNG.BaseURL,
})
```

- [ ] **Step 2: Commit**

```bash
git add cli/engine_deps.go
git commit -m "cli: wire BingMarket and SearXNGBaseURL into web search options"
```

---

## Task 9: Full Test Run & Final Verification

- [ ] **Step 1: Run the full `tool/web` test suite**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind && go test ./tool/web/... -v
```

Expected: ALL PASS.

- [ ] **Step 2: Verify project compiles**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind && go build ./...
```

Expected: clean compile, no errors.

- [ ] **Step 3: Final commit**

```bash
git commit --allow-empty -m "feat: Bing HTML crawler and SearXNG search providers complete"
```

---

## Self-Review Checklist

**1. Spec coverage:**
- Bing HTML crawler with market param → Task 4 ✅
- SearXNG JSON API with base_url → Task 5 ✅
- Auto-priority: tavily > brave > exa > searxng > bing > DuckDuckGo → Task 6 ✅
- Config structs → Task 1 ✅
- UI descriptor → Task 2 ✅
- Runtime wiring → Task 8 ✅
- Tests for both providers + dispatcher → Tasks 4, 5, 7 ✅

**2. Placeholder scan:**
- No "TBD", "TODO", "implement later" ✅
- No vague "add error handling" without code ✅
- No "similar to Task X" without repeating code ✅

**3. Type consistency:**
- `newBingProvider(market, endpoint string)` used consistently ✅
- `newSearXNGProvider(baseURL string)` used consistently ✅
- `BingMarket` / `SearXNGBaseURL` in `Options` matches config paths ✅
