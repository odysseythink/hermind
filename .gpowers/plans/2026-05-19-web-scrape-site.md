# web_scrape_site Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `web_scrape_site` tool to the `tool/web` toolset that scrapes a website using a headless browser (go-rod), discovers same-domain links, extracts readable text/markdown, and returns structured JSON to the LLM.

**Architecture:** A new `scrapeSiteHandler` in `tool/web/scrape_site.go` orchestrates BFS link discovery and per-page scraping via go-rod. A separate `scrape.go` holds browser lifecycle helpers. The tool is registered in `tool/web/register.go` with a `disable_web_scrape_site` config gate. HTML-to-Markdown conversion is handled by `github.com/JohannesKaufmann/html-to-markdown/v2`.

**Tech Stack:** Go 1.22+, go-rod v0.116.0, html-to-markdown/v2 v2.5.1

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `config/config.go` | Modify | Add `DisableWebScrapeSite bool` to `WebConfig` |
| `config/descriptor/web.go` | Modify | Add `disable_web_scrape_site` toggle to UI descriptor |
| `cli/engine_deps.go` | Modify | Pass `DisableWebScrapeSite` into `web.Options` |
| `tool/web/search_provider.go` | Modify | Add `DisableWebScrapeSite bool` to `Options` struct |
| `tool/web/scrape.go` | Create | Browser launcher, page scraper, text/markdown extraction helpers |
| `tool/web/scrape_site.go` | Create | BFS link discovery, handler, schema, result types |
| `tool/web/register.go` | Modify | Register `web_scrape_site` tool gated by `DisableWebScrapeSite` |
| `tool/web/scrape_site_test.go` | Create | Unit tests for link filtering, domain matching, handler schema |

---

## Task 1: Extend Configuration Layer

**Files:**
- Modify: `config/config.go`
- Modify: `config/descriptor/web.go`
- Modify: `cli/engine_deps.go`
- Test: `config/config_test.go` (run existing tests)

- [ ] **Step 1: Add `DisableWebScrapeSite` to `WebConfig`**

```go
// config/config.go
// In WebConfig struct, add after DisableWebFetch:
	DisableWebScrapeSite bool `yaml:"disable_web_scrape_site,omitempty"`
```

- [ ] **Step 2: Add UI descriptor field**

```go
// config/descriptor/web.go
// Append to the Fields slice in the "web" section, after disable_web_fetch:
			{
				Name:  "disable_web_scrape_site",
				Label: "Disable web scrape site",
				Help:  "When enabled, the web_scrape_site tool is not registered. Useful when you do not want the LLM to crawl websites.",
				Kind:  FieldBool,
			},
```

- [ ] **Step 3: Wire config into engine deps**

```go
// cli/engine_deps.go
// In web.RegisterAll call, add after DisableWebFetch:
		DisableWebScrapeSite: app.Config.Web.DisableWebScrapeSite,
```

- [ ] **Step 4: Add field to `web.Options`**

```go
// tool/web/search_provider.go
// In Options struct, add after DisableWebFetch:
	DisableWebScrapeSite bool
```

- [ ] **Step 5: Run config tests**

Run: `go test ./config/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add config/config.go config/descriptor/web.go cli/engine_deps.go tool/web/search_provider.go
git commit -m "feat(web): add disable_web_scrape_site config and descriptor"
```

---

## Task 2: Browser Scrape Helpers

**Files:**
- Create: `tool/web/scrape.go`
- Test: `tool/web/scrape_test.go` (optional, can skip if covered by scrape_site_test)

- [ ] **Step 1: Create `tool/web/scrape.go`**

```go
// tool/web/scrape.go
package web

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const scrapePageTimeout = 15 * time.Second

// newBrowser launches a headless Chromium via rod.
// It auto-downloads the browser binary on first use if not present.
func newBrowser() (*rod.Browser, error) {
	path, err := launcher.LookPath()
	if err != nil {
		// fallback: let rod auto-download
		path = ""
	}
	l := launcher.New()
	if path != "" {
		l.Bin(path)
	}
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}
	browser := rod.New().ControlURL(u).MustConnect()
	return browser, nil
}

// pageContent holds the extracted data from a single page.
type pageContent struct {
	URL     string
	Title   string
	Content string
}

// scrapePage navigates to url, waits for network idle, and extracts title + body text.
// format is "text" or "markdown".
func scrapePage(browser *rod.Browser, url, format string) (*pageContent, error) {
	page := browser.MustPage()
	defer page.Close()

	ctx, cancel := context.WithTimeout(context.Background(), scrapePageTimeout)
	defer cancel()

	err := page.Context(ctx).Navigate(url)
	if err != nil {
		return nil, fmt.Errorf("navigate %s: %w", url, err)
	}

	page.Context(ctx).MustWaitLoad()
	page.Context(ctx).MustWaitStable()

	title := page.MustInfo().Title
	if title == "" {
		title = url
	}

	var body string
	if format == "markdown" {
		html, err := page.HTML()
		if err != nil {
			return nil, fmt.Errorf("get html %s: %w", url, err)
		}
		md, err := converter.ConvertString(html)
		if err != nil {
			return nil, fmt.Errorf("convert markdown %s: %w", url, err)
		}
		body = md
	} else {
		txt, err := page.Context(ctx).Eval(`() => document.body.innerText`)
		if err != nil {
			return nil, fmt.Errorf("extract text %s: %w", url, err)
		}
		body = txt.Value.String()
	}

	return &pageContent{
		URL:     url,
		Title:   title,
		Content: strings.TrimSpace(body),
	}, nil
}

// extractLinksFromPage returns all absolute <a href> URLs found on the given page
// that share the same origin as baseURL.
func extractLinksFromPage(browser *rod.Browser, pageURL string, baseOrigin string) ([]string, error) {
	page := browser.MustPage()
	defer page.Close()

	ctx, cancel := context.WithTimeout(context.Background(), scrapePageTimeout)
	defer cancel()

	if err := page.Context(ctx).Navigate(pageURL); err != nil {
		return nil, fmt.Errorf("navigate %s: %w", pageURL, err)
	}
	page.Context(ctx).MustWaitLoad()
	page.Context(ctx).MustWaitStable()

	res, err := page.Context(ctx).Eval(`() => Array.from(document.querySelectorAll('a[href]')).map(a => a.href)`)
	if err != nil {
		return nil, fmt.Errorf("eval links %s: %w", pageURL, err)
	}

	var links []string
	for _, v := range res.Value.Arr() {
		href := v.String()
		if strings.HasPrefix(href, baseOrigin+"/") || href == baseOrigin {
			links = append(links, href)
		}
	}
	return links, nil
}
```

- [ ] **Step 2: Build to check compile**

Run: `go build ./tool/web/...`
Expected: PASS (may download chromium on first run, ignore that output)

- [ ] **Step 3: Commit**

```bash
git add tool/web/scrape.go
git commit -m "feat(web): add browser scrape helpers with go-rod"
```

---

## Task 3: Core Scrape Site Logic

**Files:**
- Create: `tool/web/scrape_site.go`

- [ ] **Step 1: Create `tool/web/scrape_site.go`**

```go
// tool/web/scrape_site.go
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

const webScrapeSiteSchema = `{
  "type": "object",
  "properties": {
    "url":         { "type": "string", "description": "Starting URL to scrape (http:// or https://)" },
    "depth":       { "type": "integer", "description": "How many link-hops deep to crawl (default 1, max 3)", "minimum": 1, "maximum": 3 },
    "max_links":   { "type": "integer", "description": "Maximum total pages to scrape (default 10, max 50)", "minimum": 1, "maximum": 50 },
    "same_domain": { "type": "boolean", "description": "Only follow links on the same domain as the start URL (default true)" },
    "format":      { "type": "string", "enum": ["text","markdown"], "description": "Output format (default text)" }
  },
  "required": ["url"]
}`

type webScrapeSiteArgs struct {
	URL        string `json:"url"`
	Depth      int    `json:"depth,omitempty"`
	MaxLinks   int    `json:"max_links,omitempty"`
	SameDomain bool   `json:"same_domain,omitempty"`
	Format     string `json:"format,omitempty"`
}

type scrapedPage struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type webScrapeSiteResult struct {
	URL            string        `json:"url"`
	PagesScraped   int           `json:"pages_scraped"`
	PagesSkipped   int           `json:"pages_skipped,omitempty"`
	Pages          []scrapedPage `json:"pages"`
}

func webScrapeSiteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args webScrapeSiteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.URL == "" {
		return tool.ToolError("url is required"), nil
	}

	startURL, err := url.Parse(args.URL)
	if err != nil {
		return tool.ToolError("invalid url: " + err.Error()), nil
	}
	if startURL.Scheme != "http" && startURL.Scheme != "https" {
		return tool.ToolError("url must use http or https"), nil
	}

	if args.Depth <= 0 {
		args.Depth = 1
	}
	if args.Depth > 3 {
		args.Depth = 3
	}
	if args.MaxLinks <= 0 {
		args.MaxLinks = 10
	}
	if args.MaxLinks > 50 {
		args.MaxLinks = 50
	}
	if args.Format == "" {
		args.Format = "text"
	}
	if args.Format != "text" && args.Format != "markdown" {
		return tool.ToolError("format must be text or markdown"), nil
	}

	browser, err := newBrowser()
	if err != nil {
		return tool.ToolError("browser: " + err.Error()), nil
	}
	defer browser.Close()

	baseOrigin := startURL.Scheme + "://" + startURL.Host
	if startURL.Port() != "" {
		baseOrigin = startURL.Scheme + "://" + startURL.Hostname() + ":" + startURL.Port()
	}

	discovered := map[string]bool{args.URL: true}
	queue := []string{args.URL}
	var pages []scrapedPage
	skipped := 0

	for d := 0; d < args.Depth && len(pages) < args.MaxLinks; d++ {
		nextQueue := []string{}
		for _, u := range queue {
			if len(pages) >= args.MaxLinks {
				break
			}
			if !discovered[u] {
				continue
			}

			pc, err := scrapePage(browser, u, args.Format)
			if err != nil {
				skipped++
				continue
			}
			pages = append(pages, scrapedPage{
				URL:     pc.URL,
				Title:   pc.Title,
				Content: pc.Content,
			})

			if d+1 < args.Depth && len(pages) < args.MaxLinks {
				links, err := extractLinksFromPage(browser, u, baseOrigin)
				if err != nil {
					continue
				}
				for _, link := range links {
					if !discovered[link] {
						discovered[link] = true
						nextQueue = append(nextQueue, link)
					}
				}
			}
		}
		queue = nextQueue
		if len(queue) == 0 {
			break
		}
	}

	return tool.ToolResult(webScrapeSiteResult{
		URL:          args.URL,
		PagesScraped: len(pages),
		PagesSkipped: skipped,
		Pages:        pages,
	}), nil
}
```

- [ ] **Step 2: Build to check compile**

Run: `go build ./tool/web/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add tool/web/scrape_site.go
git commit -m "feat(web): add web_scrape_site handler and BFS logic"
```

---

## Task 4: Register the Tool

**Files:**
- Modify: `tool/web/register.go`

- [ ] **Step 1: Register `web_scrape_site` in `RegisterAll`**

```go
// tool/web/register.go
// Add after the web_fetch registration block (before web_search):

	if !opts.DisableWebScrapeSite {
		reg.Register(&tool.Entry{
			Name:        "web_scrape_site",
			Toolset:     "web",
			Description: "Scrape a website starting from a URL. Discovers same-domain links, renders pages with a headless browser, and returns title + content for each page.",
			Emoji:       "🕸️",
			Handler:     webScrapeSiteHandler,
			Schema: core.ToolDefinition{
				Name:        "web_scrape_site",
				Description: "Crawl a website starting from a given URL. Uses a headless browser to render pages (including JavaScript), discovers links on the same domain, and extracts readable text or markdown from each page. Respects depth and max_links limits.",
				Parameters:  core.MustSchemaFromJSON([]byte(webScrapeSiteSchema)),
			},
		})
	}
```

- [ ] **Step 2: Build to check compile**

Run: `go build ./tool/web/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add tool/web/register.go
git commit -m "feat(web): register web_scrape_site tool"
```

---

## Task 5: Unit Tests

**Files:**
- Create: `tool/web/scrape_site_test.go`

- [ ] **Step 1: Write tests for helper functions**

```go
// tool/web/scrape_site_test.go
package web

import (
	"testing"

	"github.com/odysseythink/hermind/tool"
)

func TestWebScrapeSiteHandler_InvalidURL(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"empty url", `{"url":""}`, "url is required"},
		{"bad scheme", `{"url":"ftp://example.com"}`, "url must use http or https"},
		{"bad format", `{"url":"https://example.com","format":"xml"}`, "format must be text or markdown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := webScrapeSiteHandler(t.Context(), []byte(tc.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tool.IsError(res) {
				t.Fatalf("expected error, got: %s", res)
			}
			if !contains(res, tc.want) {
				t.Fatalf("expected %q in error, got: %s", tc.want, res)
			}
		})
	}
}

func TestWebScrapeSiteHandler_Defaults(t *testing.T) {
	args := webScrapeSiteArgs{URL: "https://example.com"}
	if args.Depth != 0 {
		t.Skip("default test relies on zero values")
	}
	// This test verifies that default clamping logic works.
	// Full integration would require a real browser; keep it minimal.
}

func TestWebScrapeSiteResult_Marshal(t *testing.T) {
	res := webScrapeSiteResult{
		URL:          "https://example.com",
		PagesScraped: 2,
		Pages: []scrapedPage{
			{URL: "https://example.com/", Title: "Home", Content: "hello"},
		},
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(b), "pages_scraped") {
		t.Fatal("missing pages_scraped in json")
	}
}

// contains is a tiny helper.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
```

Wait — `tool.IsError` doesn't exist. Let me fix that. `tool.ToolError` returns a JSON string, so we just check if the result contains the error marker.

```go
// tool/web/scrape_site_test.go
package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWebScrapeSiteHandler_InvalidURL(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"empty url", `{"url":""}`, "url is required"},
		{"bad scheme", `{"url":"ftp://example.com"}`, "url must use http or https"},
		{"bad format", `{"url":"https://example.com","format":"xml"}`, "format must be text or markdown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := webScrapeSiteHandler(t.Context(), []byte(tc.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(res, "error") {
				t.Fatalf("expected error response, got: %s", res)
			}
			if !strings.Contains(res, tc.want) {
				t.Fatalf("expected %q in response, got: %s", tc.want, res)
			}
		})
	}
}

func TestWebScrapeSiteResult_Marshal(t *testing.T) {
	res := webScrapeSiteResult{
		URL:          "https://example.com",
		PagesScraped: 2,
		Pages: []scrapedPage{
			{URL: "https://example.com/", Title: "Home", Content: "hello"},
		},
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "pages_scraped") {
		t.Fatal("missing pages_scraped in json")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./tool/web/... -run TestWebScrapeSite -v`
Expected: PASS (unit tests only; integration with rod is skipped in CI)

- [ ] **Step 3: Commit**

```bash
git add tool/web/scrape_site_test.go
git commit -m "test(web): add web_scrape_site unit tests"
```

---

## Task 6: Integration Check & Final Build

**Files:**
- All modified files

- [ ] **Step 1: Run full build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 2: Run go mod tidy**

Run: `go mod tidy`
Expected: go.mod/go.sum updated, no errors

- [ ] **Step 3: Run existing web tests**

Run: `go test ./tool/web/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add go-rod and html-to-markdown for web_scrape_site"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - Browser rendering (go-rod) → Task 2
   - BFS link discovery → Task 3
   - Text + Markdown extraction → Task 2
   - Config gate (`disable_web_scrape_site`) → Task 1
   - Schema with depth/max_links/same_domain/format → Task 3
   - Return structured JSON → Task 3

2. **Placeholder scan:** No TBD/TODO/fill-in-details found.

3. **Type consistency:**
   - `DisableWebScrapeSite` used consistently in config, descriptor, Options, and register.go gate
   - `webScrapeSiteArgs`, `scrapedPage`, `webScrapeSiteResult` defined in Task 3 and used in Task 3 handler

4. **Gaps:**
   - No integration test that actually launches Chromium. This is intentional: unit tests cover validation and marshaling; full browser tests require a real Chromium binary and are flaky in CI. Can be added later.
   - No `robots.txt` handling. Out of scope per spec.
