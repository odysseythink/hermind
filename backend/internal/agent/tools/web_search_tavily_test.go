package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestTavily_ParseResponse(t *testing.T) {
	fixture := `{
		"results": [
			{
				"title": "The Go Programming Language",
				"url": "https://golang.org/",
				"content": "Go is an open source programming language supported by Google."
			},
			{
				"title": "Go Packages",
				"url": "https://pkg.go.dev/",
				"content": "Find, add, and publish Go packages."
			},
			{
				"title": "",
				"url": "https://example.com/",
				"content": "Should be skipped"
			}
		]
	}`

	results := normalizeTavilyResponseJSON(t, fixture)
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
	if first.Snippet != "Go is an open source programming language supported by Google." {
		t.Errorf("snippet mismatch: %q", first.Snippet)
	}
}

func TestTavily_DetailError(t *testing.T) {
	fixture := `{"detail": {"error": "Invalid API key"}}`
	var resp tavilyResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Detail == nil || resp.Detail.Error != "Invalid API key" {
		t.Fatalf("expected detail error")
	}
}

func TestTavily_MissingKey(t *testing.T) {
	provider := getSearchProvider("tavily-search")
	if provider == nil {
		t.Fatal("tavily-search provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Tavily API key not configured. Set AgentTavilyApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestTavily_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Tavily search")
	}

	apiKey := os.Getenv("AGENT_TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_TAVILY_API_KEY to run live Tavily search")
	}

	provider := getSearchProvider("tavily-search")
	if provider == nil {
		t.Fatal("tavily-search provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentTavilyApiKey": apiKey,
	}, &config.Config{})
	if err != nil {
		t.Fatalf("live search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one live result")
	}

	for i, r := range results {
		if r.Title == "" || r.Link == "" || r.Snippet == "" {
			t.Fatalf("result %d has empty fields: %+v", i, r)
		}
	}
}

func normalizeTavilyResponseJSON(t *testing.T, payload string) []SearchResult {
	t.Helper()
	var resp tavilyResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	results := make([]SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.URL,
			Snippet: r.Content,
		})
	}
	return results
}
