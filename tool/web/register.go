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
