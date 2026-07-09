package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestSerply_ParseResponse(t *testing.T) {
	fixture := `{
		"results": [
			{
				"title": "The Go Programming Language",
				"link": "https://golang.org/",
				"description": "Go is an open source programming language."
			},
			{
				"title": "Go Packages",
				"link": "https://pkg.go.dev/",
				"description": "Find, add, and publish Go packages."
			}
		]
	}`

	var resp serplyResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := normalizeSerplyResults(resp.Results)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "The Go Programming Language" {
		t.Errorf("title mismatch: %+v", results[0])
	}
}

func TestSerply_MissingKey(t *testing.T) {
	provider := getSearchProvider("serply-engine")
	if provider == nil {
		t.Fatal("serply-engine provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Serply API key not configured. Set AgentSerplyApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestSerply_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Serply search")
	}

	apiKey := os.Getenv("AGENT_SERPLY_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_SERPLY_API_KEY to run live Serply search")
	}

	provider := getSearchProvider("serply-engine")
	if provider == nil {
		t.Fatal("serply-engine provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentSerplyApiKey": apiKey,
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

func normalizeSerplyResults(items []serplyResult) []SearchResult {
	results := make([]SearchResult, 0, len(items))
	for _, r := range items {
		if r.Title == "" || r.Link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.Link,
			Snippet: r.Description,
		})
	}
	return results
}
