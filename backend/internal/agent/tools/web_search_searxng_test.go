package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestSearXNG_ParseResponse(t *testing.T) {
	fixture := `{
		"results": [
			{
				"title": "The Go Programming Language",
				"url": "https://golang.org/",
				"content": "Go is an open source programming language.",
				"publishedDate": "2024-01-01"
			},
			{
				"title": "Go Packages",
				"url": "https://pkg.go.dev/",
				"content": "Find, add, and publish Go packages."
			}
		]
	}`

	var resp searxngResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := normalizeSearXNGResults(resp.Results)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].PublishedDate != "2024-01-01" {
		t.Errorf("publishedDate mismatch: %q", results[0].PublishedDate)
	}
}

func TestSearXNG_MissingURL(t *testing.T) {
	provider := getSearchProvider("searxng-engine")
	if provider == nil {
		t.Fatal("searxng-engine provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing base URL")
	}
	if err.Error() != "SearXNG base URL not configured. Set AgentSearXNGApiUrl in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestSearXNG_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live SearXNG search")
	}

	baseURL := os.Getenv("AGENT_SEARXNG_API_URL")
	if baseURL == "" {
		t.Skip("set AGENT_SEARXNG_API_URL to run live SearXNG search")
	}

	provider := getSearchProvider("searxng-engine")
	if provider == nil {
		t.Fatal("searxng-engine provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentSearXNGApiUrl": baseURL,
	}, &config.Config{})
	if err != nil {
		t.Fatalf("live search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one live result")
	}

	for i, r := range results {
		if r.Title == "" || r.Link == "" {
			t.Fatalf("result %d has empty title/link: %+v", i, r)
		}
	}
}

func normalizeSearXNGResults(items []searxngResult) []SearchResult {
	results := make([]SearchResult, 0, len(items))
	for _, r := range items {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:         r.Title,
			Link:          r.URL,
			Snippet:       r.Content,
			PublishedDate: r.PublishedDate,
		})
	}
	return results
}
