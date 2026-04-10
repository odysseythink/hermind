// tool/web/register.go
package web

import (
	"encoding/json"

	"github.com/nousresearch/hermes-agent/tool"
)

// RegisterAll registers the web tools into a registry.
// - web_fetch is always registered (uses stdlib http)
// - web_search is registered only if exaAPIKey is non-empty
// - web_extract is registered only if firecrawlAPIKey is non-empty
func RegisterAll(reg *tool.Registry, exaAPIKey, firecrawlAPIKey string) {
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

	if exaAPIKey != "" {
		reg.Register(&tool.Entry{
			Name:        "web_search",
			Toolset:     "web",
			Description: "Search the web via Exa.",
			Emoji:       "🔎",
			Handler:     newWebSearchHandler(exaAPIKey, ""),
			Schema: tool.ToolDefinition{
				Type: "function",
				Function: tool.FunctionDef{
					Name:        "web_search",
					Description: "Search the web and return a list of results.",
					Parameters:  json.RawMessage(webSearchSchema),
				},
			},
		})
	}

	if firecrawlAPIKey != "" {
		reg.Register(&tool.Entry{
			Name:        "web_extract",
			Toolset:     "web",
			Description: "Extract page content as markdown/html/text via Firecrawl.",
			Emoji:       "📰",
			Handler:     newWebExtractHandler(firecrawlAPIKey, ""),
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
