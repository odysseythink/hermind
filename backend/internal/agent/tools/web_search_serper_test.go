package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestSerper_ParseResponse(t *testing.T) {
	fixture := `{
		"knowledgeGraph": {
			"title": "Go",
			"description": "Go is a statically typed, compiled programming language designed at Google."
		},
		"organic": [
			{
				"title": "The Go Programming Language",
				"link": "https://golang.org/",
				"snippet": "Go is an open source programming language supported by Google."
			},
			{
				"title": "Go Packages",
				"link": "https://pkg.go.dev/",
				"snippet": "Find, add, and publish Go packages."
			},
			{
				"title": "",
				"link": "https://example.com/",
				"snippet": "Should be skipped due to empty title"
			}
		]
	}`

	results := normalizeSerperResponseJSON(t, fixture)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	kg := results[0]
	if kg.Title != "Go" {
		t.Errorf("knowledge graph title: expected %q, got %q", "Go", kg.Title)
	}
	if kg.Link != "" {
		t.Errorf("knowledge graph link: expected empty, got %q", kg.Link)
	}
	if kg.Snippet != "Go is a statically typed, compiled programming language designed at Google." {
		t.Errorf("knowledge graph snippet mismatch: %q", kg.Snippet)
	}

	first := results[1]
	if first.Title != "The Go Programming Language" {
		t.Errorf("first organic title: expected %q, got %q", "The Go Programming Language", first.Title)
	}
	if first.Link != "https://golang.org/" {
		t.Errorf("first organic link: expected %q, got %q", "https://golang.org/", first.Link)
	}
}

func TestSerper_MissingKey(t *testing.T) {
	provider := getSearchProvider("serper-dot-dev")
	if provider == nil {
		t.Fatal("serper-dot-dev provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Serper.dev API key not configured. Set AgentSerperApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestSerper_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Serper search")
	}

	apiKey := os.Getenv("AGENT_SERPER_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_SERPER_API_KEY to run live Serper search")
	}

	provider := getSearchProvider("serper-dot-dev")
	if provider == nil {
		t.Fatal("serper-dot-dev provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentSerperApiKey": apiKey,
	}, &config.Config{})
	if err != nil {
		t.Fatalf("live search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one live result")
	}

	for i, r := range results {
		if r.Title == "" || r.Snippet == "" {
			t.Fatalf("result %d has empty title/snippet: %+v", i, r)
		}
	}
}

func normalizeSerperResponseJSON(t *testing.T, payload string) []SearchResult {
	t.Helper()
	var resp serperResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return normalizeSerperResponse(&resp)
}
