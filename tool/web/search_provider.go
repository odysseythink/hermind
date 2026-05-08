package web

import (
	"context"
	"time"

	"github.com/odysseythink/hermind/config"
)

// httpTimeout is the per-request timeout every provider applies to its
// outbound HTTP call. Increased to 60s to account for proxy latency. Most
// providers complete in <10s direct, but proxy adds overhead.
const httpTimeout = 60 * time.Second

// SearchProvider is the contract every web_search backend implements.
type SearchProvider interface {
	ID() string
	Configured() bool
	Search(ctx context.Context, q string, n int) ([]SearchResult, error)
}

// SearchResult is the normalized shape emitted to the LLM.
type SearchResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Snippet       string   `json:"snippet"`
	PublishedDate string   `json:"published_date,omitempty"`
	Score         *float64 `json:"score,omitempty"`
}

// Options is the flat parameter bundle consumed by RegisterAll.
type Options struct {
	SearchProvider    string
	TavilyAPIKey      string
	BraveAPIKey       string
	ExaAPIKey         string
	FirecrawlAPIKey   string
	DDGProxyConfig    *config.DDGProxyConfig
	BingMarket        string
	SearXNGBaseURL    string
	DisableWebFetch   bool
	DefaultNumResults int
	MaxNumResults     int
}
