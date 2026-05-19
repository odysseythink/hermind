# Design: web_scrape_site Tool

Date: 2026-05-19
Topic: web-scrape-site

## 1. Goal

Add a new `web_scrape_site` tool to the `tool/web` toolset that lets the LLM
scrape a website starting from a given URL, discover same-domain links, render
pages with a headless browser, extract readable text, and return the collected
pages back to the LLM as structured JSON.

This aligns with the AnythingLLM "抓取网站" Agent Skill but is implemented
as a native hermind tool using go-rod for browser rendering.

## 2. User-Confirmed Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Result handling | **A: Pure return** (no auto-save) | Fits hermind's tool philosophy: give LLM the data, let it decide what to do with it. LLM can call `write_file` if it wants persistence. |
| Rendering engine | **B: Browser rendering (go-rod)** | Can handle JS-driven SPAs. go-rod auto-downloads Chromium on first use, no manual install required on most platforms. |

## 3. Architecture

```
LLM tool call
    ↓
webScrapeSiteHandler (tool/web/scrape_site.go)
    ↓
scrapeSite(ctx, args)
    ├── discoverLinks(startURL, depth, maxLinks, sameDomain)
    │   ├── rod.Page.Navigate(url)
    │   ├── rod.Page.Element("body").Text()   → page text for extraction
    │   ├── parse HTML for <a href="...">
    │   └── filter: same domain, dedup, respect maxLinks
    │   └── repeat for each depth level (BFS)
    └── scrapePage(pageURL) for each discovered link
        ├── rod.Page.Navigate(url)
        ├── wait for "networkidle"
        ├── extract title (document.title)
        ├── extract body text (remove script/style/nav/footer via rod/JS eval)
        └── close page
    ↓
return JSON: [{"url","title","content"}, ...]
```

## 4. Tool Schema

```json
{
  "type": "object",
  "properties": {
    "url":         { "type": "string", "description": "Starting URL to scrape (http:// or https://)" },
    "depth":       { "type": "integer", "description": "How many link-hops deep to crawl (default 1, max 3)", "minimum": 1, "maximum": 3 },
    "max_links":   { "type": "integer", "description": "Maximum total pages to scrape (default 10, max 50)", "minimum": 1, "maximum": 50 },
    "same_domain": { "type": "boolean", "description": "Only follow links on the same domain as the start URL (default true)" },
    "format":      { "type": "string", "enum": ["text","markdown"], "description": "Output format (default text)" }
  },
  "required": ["url"]
}
```

## 5. Return Value

```json
{
  "url": "https://example.com",
  "pages_scraped": 3,
  "pages": [
    {
      "url": "https://example.com/",
      "title": "Example Domain",
      "content": "This domain is for use in illustrative examples..."
    },
    {
      "url": "https://example.com/docs",
      "title": "Documentation",
      "content": "..."
    }
  ]
}
```

## 6. Implementation Files

| File | Action | Description |
|------|--------|-------------|
| `tool/web/scrape_site.go` | **Create** | Core logic: link discovery, page scraping, text extraction |
| `tool/web/scrape.go` | **Create** | rod launcher helpers, browser lifecycle management |
| `tool/web/register.go` | **Modify** | Register `web_scrape_site` tool; add `DisableWebScrapeSite` gate |
| `tool/web/search_provider.go` | **Modify** | Add `DisableWebScrapeSite bool` to `Options` struct |
| `cli/engine_deps.go` | **Modify** | Pass `DisableWebScrapeSite` from config to `web.RegisterAll` |
| `config/config.go` | **Modify** | Add `DisableWebScrapeSite bool` to `WebConfig` |
| `config/descriptor/web.go` | **Modify** | Add `disable_web_scrape_site` UI field |
| `tool/web/scrape_site_test.go` | **Create** | Unit tests for link filtering, domain matching, text extraction |
| `go.mod` / `go.sum` | **Modify** | Add `github.com/go-rod/rod` and `github.com/JohannesKaufmann/html-to-markdown` dependencies |

## 7. Configuration

- **Environment:** No new env vars needed. rod auto-downloads Chromium.
- **Config field:** `web.disable_web_scrape_site` (bool, default false)
- **UI:** Add a toggle in Settings → Web tools, same pattern as `disable_web_fetch`.

## 8. Browser Lifecycle & Resource Management

- One `rod.Browser` instance is launched per `web_scrape_site` call.
- Pages are created, scraped, and closed individually.
- The browser is closed after all pages are processed or on error.
- Timeout: overall tool timeout = `depth * max_links * 15s` capped at 5 minutes.

## 9. Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid URL | Return tool error immediately |
| rod/Chromium not available | Return tool error with install hint |
| Page load timeout | Skip page, include in skipped count, continue |
| Empty content | Include page with empty content (LLM decides) |
| Depth = 0 | Treat as 1 (minimum) |

## 10. Text Extraction Strategy

Because we use a real browser, we can use `page.Eval` to run a JavaScript
readability-like snippet that returns the main article text, falling back to
`document.body.innerText` if the page has no clear article structure.

```javascript
// Pseudo JS snippet evaluated in page
function extractText() {
  const article = document.querySelector('article, main, [role="main"]');
  if (article) return article.innerText;
  return document.body.innerText;
}
```

For `format: "markdown"`, the page HTML is piped through
`github.com/JohannesKaufmann/html-to-markdown` to produce clean Markdown.
This is added as a new dependency alongside rod.

## 11. Security & Guardrails

- `same_domain` defaults to `true` to prevent accidentally crawling the entire
  internet.
- `max_links` capped at 50 to avoid runaway context explosion.
- `depth` capped at 3.
- Respect `robots.txt` is **out of scope** for MVP (same as AnythingLLM).

## 12. Testing Strategy

| Test | Type | Description |
|------|------|-------------|
| `TestScrapeSiteLinkFilter` | unit | URL same-domain filtering logic |
| `TestScrapeSiteLinkDiscovery` | unit | HTML → link extraction |
| `TestScrapeSiteMaxLinksCap` | unit | Stop BFS when max_links reached |
| `TestScrapeSiteHandlerSchema` | unit | Invalid args return tool error |
| `TestScrapeSiteIntegration` | integration (optional) | Spin up rod against a local HTTP test server |

## 13. Future Extensions

- `same_domain: false` with explicit allow-list / block-list.
- `robots.txt` compliance.
- Save-to-file mode (auto-write `.md` files to workspace) if users request it.
- Re-use a shared browser pool across tool calls to amortize launch cost.
