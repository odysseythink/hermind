package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestExa_ParseResponse(t *testing.T) {
	fixture := `{
		"results": [
			{
				"title": "The Go Programming Language",
				"url": "https://golang.org/",
				"text": "Go is an open source programming language supported by Google.",
				"publishedDate": "2024-01-01"
			},
			{
				"title": "Go Packages",
				"url": "https://pkg.go.dev/",
				"text": "Find, add, and publish Go packages."
			},
			{
				"title": "",
				"url": "https://example.com/",
				"text": "Should be skipped"
			}
		]
	}`

	results := normalizeExaResponseJSON(t, fixture)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	first := results[0]
	if first.Title != "The Go Programming Language" {
		t.Errorf("title: expected %q, got %q", "The Go Programming Language", first.Title)
	}
	if first.Link != "https://golang.org/" {
		t.Errorf("link: expected %q, got %q", "https://golang.org/", first.Link)
	}
	if first.PublishedDate != "2024-01-01" {
		t.Errorf("publishedDate: expected %q, got %q", "2024-01-01", first.PublishedDate)
	}

	second := results[1]
	if second.PublishedDate != "" {
		t.Errorf("expected empty publishedDate for second result, got %q", second.PublishedDate)
	}
}

func TestExa_MissingKey(t *testing.T) {
	provider := getSearchProvider("exa-search")
	if provider == nil {
		t.Fatal("exa-search provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Exa API key not configured. Set AgentExaApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestExa_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Exa search")
	}

	apiKey := os.Getenv("AGENT_EXA_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_EXA_API_KEY to run live Exa search")
	}

	provider := getSearchProvider("exa-search")
	if provider == nil {
		t.Fatal("exa-search provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentExaApiKey": apiKey,
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

func normalizeExaResponseJSON(t *testing.T, payload string) []SearchResult {
	t.Helper()
	var resp exaResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	results := make([]SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:         r.Title,
			Link:          r.URL,
			Snippet:       r.Text,
			PublishedDate: r.PublishedDate,
		})
	}
	return results
}
