// tool/web/register.go
package web

import (
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// RegisterAll wires the web toolset into reg according to opts.
//
//   - web_fetch is always registered.
//   - web_scrape_site is always registered.
//   - web_search is always registered; the dispatcher chooses a provider
//     based on opts.SearchProvider or built-in priority. DuckDuckGo is the
//     keyless fallback so this tool is never unavailable.
//   - web_extract is registered only when opts.FirecrawlAPIKey is
//     non-empty.
func RegisterAll(reg *tool.Registry, opts Options) {
	reg.Register(&tool.Entry{
		Name:        "web_fetch",
		Toolset:     "web",
		Description: "Fetch a URL and return status + headers + body (max 2 MiB).",
		Emoji:       "🌐",
		Handler:     webFetchHandler,
		Schema: core.ToolDefinition{
			Name:        "web_fetch",
			Description: "Perform an HTTP GET/POST to a URL and return the response.",
			Parameters:  core.MustSchemaFromJSON([]byte(webFetchSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "web_scrape_site",
		Toolset:     "web",
		Description: "Scrape a website starting from a URL. Discovers links, renders pages with a headless browser, and returns title + content for each page.",
		Emoji:       "🕸️",
		Handler:     webScrapeSiteHandler,
		Schema: core.ToolDefinition{
			Name:        "web_scrape_site",
			Description: "Crawl a website starting from a given URL. Uses a headless browser to render pages (including JavaScript), discovers links, and extracts readable text or markdown from each page. Respects depth and max_links limits.",
			Parameters:  core.MustSchemaFromJSON([]byte(webScrapeSiteSchema)),
		},
	})

	dispatcher := newSearchDispatcher(opts)
	reg.Register(&tool.Entry{
		Name:        "web_search",
		Toolset:     "web",
		Description: "Search the web via a configured provider (Tavily, Brave, Exa, SearXNG, Bing, or DuckDuckGo).",
		Emoji:       "🔎",
		Handler:     dispatcher.Handler(),
		Schema: core.ToolDefinition{
			Name:        "web_search",
			Description: "Search the web and return a list of results.",
			Parameters:  core.MustSchemaFromJSON([]byte(webSearchSchema)),
		},
	})

	if opts.FirecrawlAPIKey != "" {
		reg.Register(&tool.Entry{
			Name:        "web_extract",
			Toolset:     "web",
			Description: "Extract page content as markdown/html/text via Firecrawl.",
			Emoji:       "📰",
			Handler:     newWebExtractHandler(opts.FirecrawlAPIKey, ""),
			Schema: core.ToolDefinition{
				Name:        "web_extract",
				Description: "Extract the main content of a web page.",
				Parameters:  core.MustSchemaFromJSON([]byte(webExtractSchema)),
			},
		})
	}
}
