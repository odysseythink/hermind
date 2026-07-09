package tools

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/config"
)

// SearchResult is the normalized output across all search providers.
// Matches the JSON shape the AnythingLLM frontend expects.
type SearchResult struct {
	Title         string `json:"title"`
	Link          string `json:"link"`
	Snippet       string `json:"snippet"`
	PublishedDate string `json:"publishedDate,omitempty"`
}

// Citation is a single source link delivered to the frontend via WebSocket
// after a web search. Matches the shape the AnythingLLM frontend expects.
type Citation struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Text        string  `json:"text"`
	ChunkSource string  `json:"chunkSource"`
	Score       *string `json:"score,omitempty"`
}

// SearchProvider executes a web search and returns normalized results.
// Each provider (DuckDuckGo, Serper, Tavily, etc.) implements this once.
// Settings and cfg are passed per-request so API keys can be hot-reloaded.
type SearchProvider interface {
	Name() string
	Search(ctx context.Context, query string, settings map[string]string, cfg *config.Config) ([]SearchResult, error)
}
