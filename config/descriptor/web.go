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
	// Gate api_key fields on the selected provider. The "" (auto-select)
	// case also reveals all three so users can pre-populate keys before
	// committing to one — auto-select picks the first provider with a
	// configured key by priority (Tavily > Brave > Exa > SearXNG > Bing > DuckDuckGo).
	gate := func(provider string) *Predicate {
		return &Predicate{Field: "search.provider", In: []any{"", provider}}
	}
	// Gate DDG proxy fields on search.provider selection (visible on "" or "DuckDuckGo")
	ddgGate := func(provider string) *Predicate {
		return &Predicate{Field: "search.provider", In: []any{"", provider}}
	}
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
				Help:  "Leave blank to auto-select by priority (Tavily > Brave > Exa > SearXNG > Bing > DuckDuckGo).",
				Kind:  FieldEnum,
				Enum:  []string{"tavily", "brave", "exa", "searxng", "bing", "DuckDuckGo"},
			},
			{
				Name:        "search.providers.tavily.api_key",
				Label:       "Tavily API key",
				Kind:        FieldSecret,
				Help:        "Env var TAVILY_API_KEY overrides this value at runtime.",
				VisibleWhen: gate("tavily"),
			},
			{
				Name:        "search.providers.brave.api_key",
				Label:       "Brave Search API key",
				Kind:        FieldSecret,
				Help:        "Env var BRAVE_API_KEY overrides this value at runtime.",
				VisibleWhen: gate("brave"),
			},
			{
				Name:        "search.providers.exa.api_key",
				Label:       "Exa API key",
				Kind:        FieldSecret,
				Help:        "Env var EXA_API_KEY overrides this value at runtime.",
				VisibleWhen: gate("exa"),
			},
			{
				Name:        "search.providers.bing.market",
				Label:       "Bing market",
				Kind:        FieldString,
				Help:        "Market code for Bing results, e.g. zh-CN, en-US. Leave blank for default.",
				VisibleWhen: gate("bing"),
			},
			{
				Name:        "search.providers.searxng.base_url",
				Label:       "SearXNG base URL",
				Kind:        FieldString,
				Help:        "Base URL of your SearXNG instance, e.g. http://localhost:8080.",
				VisibleWhen: gate("searxng"),
			},
			{
				Name:        "search.providers.duckduckgo.url",
				Label:       "Proxy URL",
				Help:        "Proxy endpoint URL (e.g., http://proxy.corp.com:8080 or socks5://proxy:1080). Leave blank to disable.",
				Kind:        FieldString,
				VisibleWhen: ddgGate("DuckDuckGo"),
			},
			{
				Name:        "search.providers.duckduckgo.username",
				Label:       "Proxy username",
				Help:        "Optional proxy authentication username.",
				Kind:        FieldString,
				VisibleWhen: ddgGate("DuckDuckGo"),
			},
			{
				Name:        "search.providers.duckduckgo.password",
				Label:       "Proxy password",
				Help:        "Optional proxy authentication password.",
				Kind:        FieldSecret,
				VisibleWhen: ddgGate("DuckDuckGo"),
			},
		},
	})
}
