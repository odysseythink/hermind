package web

import (
	"context"
	"time"
)

// httpTimeout is the per-request timeout every provider applies to its
// outbound HTTP call.
const httpTimeout = 30 * time.Second

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
	SearchProvider  string
	TavilyAPIKey    string
	BraveAPIKey     string
	ExaAPIKey       string
	FirecrawlAPIKey string
}
