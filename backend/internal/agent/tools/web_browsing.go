package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

const defaultSearchProvider = "duckduckgo-engine"

// NewWebBrowsingSkill returns the agent tool entry that searches the web.
func NewWebBrowsingSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "web-browsing",
		Toolset:        "web",
		Description:    "Search the web for real-time information. Returns a list of results with title, link, and snippet.",
		Emoji:          "🌐",
		MaxResultChars: 16 * 1024,
		Schema: core.ToolDefinition{
			Name:        "web-browsing",
			Description: "Search the internet for up-to-date information",
			Parameters:  webBrowsingSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(raw, &args); err != nil || args.Query == "" {
				return tool.Error("query is required"), nil
			}

			providerKey := tc.Settings["agent_search_provider"]
			if providerKey == "" {
				providerKey = defaultSearchProvider
			}
			p := getSearchProvider(providerKey)
			if p == nil {
				providerKey = defaultSearchProvider
				p = getSearchProvider(providerKey)
			}

			tc.Emit(fmt.Sprintf("Searching web via %s for: %s", p.Name(), args.Query))
			results, err := p.Search(ctx, args.Query, tc.Settings, tc.Cfg)
			if err != nil {
				// If the configured provider is not the default and failed,
				// try falling back to DuckDuckGo.
				if providerKey != defaultSearchProvider {
					tc.Emit(fmt.Sprintf("Search via %s failed: %s. Falling back to DuckDuckGo.", p.Name(), err))
					fb := getSearchProvider(defaultSearchProvider)
					if fb != nil {
						results, err = fb.Search(ctx, args.Query, tc.Settings, tc.Cfg)
						if err != nil {
							return tool.Error("Web search is currently unavailable. All search providers failed."), nil
						}
						// Fallback succeeded — continue with results below.
					} else {
						return tool.Error("Web search is currently unavailable. No fallback provider registered."), nil
					}
				} else {
					return tool.Error(fmt.Sprintf("Web search is currently unavailable. %s failed: %s", p.Name(), err)), nil
				}
			}
			if len(results) == 0 {
				return tool.Result("No information was found online for the search query."), nil
			}

			// Emit citations so the frontend renders clickable source links.
			if tc.EmitCitations != nil {
				citations := make([]Citation, 0, len(results))
				for _, r := range results {
					if r.Link == "" {
						continue
					}
					citations = append(citations, Citation{
						ID:          r.Link,
						Title:       r.Title,
						Text:        r.Snippet,
						ChunkSource: "link://" + r.Link,
					})
				}
				if len(citations) > 0 {
					tc.EmitCitations(citations)
				}
			}

			tc.Emit(fmt.Sprintf("Found %d results via %s", len(results), p.Name()))
			return tool.Result(results), nil
		},
	}
}

func webBrowsingSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query"
			}
		},
		"required": ["query"]
	}`))
}
