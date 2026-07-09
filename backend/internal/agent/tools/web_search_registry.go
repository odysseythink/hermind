package tools

// Provider keys match frontend WebSearchSelection values exactly:
//   duckduckgo-engine, brave-search, serpapi, searchapi, serper-dot-dev,
//   bing-search, baidu-search, serply-engine, searxng-engine, tavily-search,
//   exa-search, perplexity-search, crw-search

var searchProviderRegistry = map[string]SearchProvider{}

func registerSearchProvider(key string, p SearchProvider) {
	searchProviderRegistry[key] = p
}

func getSearchProvider(key string) SearchProvider {
	return searchProviderRegistry[key]
}
