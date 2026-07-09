package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestBing_ParseResponse(t *testing.T) {
	fixture := `{
		"webPages": {
			"value": [
				{
					"name": "The Go Programming Language",
					"url": "https://golang.org/",
					"snippet": "Go is an open source programming language supported by Google."
				},
				{
					"name": "Go Packages",
					"url": "https://pkg.go.dev/",
					"snippet": "Find, add, and publish Go packages."
				},
				{
					"name": "",
					"url": "https://example.com/",
					"snippet": "Should be skipped"
				}
			]
		}
	}`

	results := normalizeBingResponseJSON(t, fixture)
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
}

func TestBing_MissingKey(t *testing.T) {
	provider := getSearchProvider("bing-search")
	if provider == nil {
		t.Fatal("bing-search provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Bing Search API key not configured. Set AgentBingSearchApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestBing_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Bing search")
	}

	apiKey := os.Getenv("AGENT_BING_SEARCH_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_BING_SEARCH_API_KEY to run live Bing search")
	}

	provider := getSearchProvider("bing-search")
	if provider == nil {
		t.Fatal("bing-search provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentBingSearchApiKey": apiKey,
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

func normalizeBingResponseJSON(t *testing.T, payload string) []SearchResult {
	t.Helper()
	var resp bingResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	results := make([]SearchResult, 0, len(resp.WebPages.Value))
	for _, r := range resp.WebPages.Value {
		if r.Name == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Name,
			Link:    r.URL,
			Snippet: r.Snippet,
		})
	}
	return results
}
