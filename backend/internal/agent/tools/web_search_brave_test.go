package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestBrave_ParseResponse(t *testing.T) {
	fixture := `{
		"web": {
			"results": [
				{
					"title": "The Go Programming Language",
					"url": "https://golang.org/",
					"description": "Go is an open source programming language supported by Google."
				},
				{
					"title": "Go Packages",
					"url": "https://pkg.go.dev/",
					"description": "Find, add, and publish Go packages."
				},
				{
					"title": "",
					"url": "https://example.com/",
					"description": "Should be skipped"
				}
			]
		}
	}`

	results := normalizeBraveResponseJSON(t, fixture)
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

func TestBrave_MissingKey(t *testing.T) {
	provider := getSearchProvider("brave-search")
	if provider == nil {
		t.Fatal("brave-search provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Brave Search API key not configured. Set AgentBraveApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestBrave_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Brave search")
	}

	apiKey := os.Getenv("AGENT_BRAVE_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_BRAVE_API_KEY to run live Brave search")
	}

	provider := getSearchProvider("brave-search")
	if provider == nil {
		t.Fatal("brave-search provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentBraveApiKey": apiKey,
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

func normalizeBraveResponseJSON(t *testing.T, payload string) []SearchResult {
	t.Helper()
	var resp braveResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	results := make([]SearchResult, 0, len(resp.Web.Results))
	for _, r := range resp.Web.Results {
		if r.Title == "" || r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.URL,
			Snippet: r.Description,
		})
	}
	return results
}
