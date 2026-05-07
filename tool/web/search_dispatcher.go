package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/hermind/tool"
)

type cacheEntry struct {
	value     []SearchResult
	expiresAt time.Time
}

// searchCache is a bounded, TTL-aware LRU. All operations take a single
// mutex. order is the eviction queue: index 0 is the LRU slot, the last
// index is the MRU slot.
type searchCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	order   []string
	maxSize int
	ttl     time.Duration
}

func newSearchCache(maxSize int, ttl time.Duration) *searchCache {
	return &searchCache{
		entries: make(map[string]cacheEntry, maxSize),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (c *searchCache) Get(key string) ([]SearchResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		c.removeLocked(key)
		return nil, false
	}
	c.touchLocked(key)
	return e.value, true
}

func (c *searchCache) Set(key string, value []SearchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; exists {
		c.entries[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
		c.touchLocked(key)
		return
	}
	if len(c.entries) >= c.maxSize && len(c.order) > 0 {
		c.removeLocked(c.order[0])
	}
	c.entries[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.order = append(c.order, key)
}

func (c *searchCache) removeLocked(key string) {
	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

func (c *searchCache) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}

// searchDispatcher picks a SearchProvider based on configuration,
// checks the cache, and runs the chosen provider. It exposes a
// tool.Handler shape via Handler() so the tool.Registry can invoke it
// directly.
type searchDispatcher struct {
	providers map[string]SearchProvider
	explicit  string // opts.SearchProvider — "" means auto-priority
	cache     *searchCache
}

// priorityOrder is the auto-select sequence. DuckDuckGo is always last
// because it is the keyless fallback.
var priorityOrder = []string{"tavily", "brave", "exa", "searxng", "bing", "DuckDuckGo"}

// newSearchDispatcher constructs a dispatcher from the caller's
// Options. DuckDuckGo is always registered; the other three are registered
// regardless of key presence and Configured() reports the real state.
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

func (d *searchDispatcher) resolveProvider() (SearchProvider, error) {
	if d.explicit != "" {
		p, ok := d.providers[d.explicit]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", d.explicit)
		}
		if !p.Configured() {
			return nil, fmt.Errorf("provider %q not configured", d.explicit)
		}
		return p, nil
	}
	for _, id := range priorityOrder {
		if p, ok := d.providers[id]; ok && p.Configured() {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no search provider configured")
}

// searchArgs is the dispatcher's input shape.
type searchArgs struct {
	Query      string `json:"query"`
	NumResults int    `json:"num_results,omitempty"`
}

type webSearchPayload struct {
	Query    string         `json:"query"`
	Provider string         `json:"provider"`
	Results  []SearchResult `json:"results"`
}

// Handler returns a tool.Handler that runs the dispatcher pipeline:
// parse args → resolveProvider → cache.Get → provider.Search →
// normalize → cache.Set.
func (d *searchDispatcher) Handler() tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args searchArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Query == "" {
			return tool.ToolError("query is required"), nil
		}
		n := args.NumResults
		if n <= 0 {
			n = 5
		}
		if n > 20 {
			n = 20
		}

		provider, err := d.resolveProvider()
		if err != nil {
			log.Printf("[web_search] resolve err=%v", err)
			return tool.ToolError(err.Error()), nil
		}

		cacheKey := fmt.Sprintf("%s|%s|%d", provider.ID(), strings.ToLower(args.Query), n)
		if cached, ok := d.cache.Get(cacheKey); ok {
			return tool.ToolResult(webSearchPayload{
				Query:    args.Query,
				Provider: provider.ID(),
				Results:  cached,
			}), nil
		}

		results, err := provider.Search(ctx, args.Query, n)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			log.Printf("[web_search] provider=%s err=%v", provider.ID(), err)
			return tool.ToolError(provider.ID() + ": " + err.Error()), nil
		}

		d.cache.Set(cacheKey, results)
		return tool.ToolResult(webSearchPayload{
			Query:    args.Query,
			Provider: provider.ID(),
			Results:  results,
		}), nil
	}
}
