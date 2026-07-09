package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestPerplexity_ParseResponse(t *testing.T) {
	fixture := `{
		"results": [
			{
				"title": "The Go Programming Language",
				"url": "https://golang.org/",
				"snippet": "Go is an open source programming language supported by Google."
			},
			{
				"name": "Go Packages",
				"link": "https://pkg.go.dev/",
				"text": "Find, add, and publish Go packages."
			},
			{
				"title": "",
				"url": "https://example.com/",
				"snippet": "Should be skipped"
			}
		]
	}`

	results := normalizePerplexityResponseJSON(t, fixture)
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

	second := results[1]
	if second.Title != "Go Packages" {
		t.Errorf("title: expected %q, got %q", "Go Packages", second.Title)
	}
	if second.Link != "https://pkg.go.dev/" {
		t.Errorf("link: expected %q, got %q", "https://pkg.go.dev/", second.Link)
	}
}

func TestPerplexity_MissingKey(t *testing.T) {
	provider := getSearchProvider("perplexity-search")
	if provider == nil {
		t.Fatal("perplexity-search provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Perplexity API key not configured. Set AgentPerplexityApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestPerplexity_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Perplexity search")
	}

	apiKey := os.Getenv("AGENT_PERPLEXITY_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_PERPLEXITY_API_KEY to run live Perplexity search")
	}

	provider := getSearchProvider("perplexity-search")
	if provider == nil {
		t.Fatal("perplexity-search provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentPerplexityApiKey": apiKey,
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

func normalizePerplexityResponseJSON(t *testing.T, payload string) []SearchResult {
	t.Helper()
	var resp perplexityResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	results := make([]SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		title := firstNonEmptyString(r.Title, r.Name)
		link := firstNonEmptyString(r.URL, r.Link)
		if title == "" || link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   title,
			Link:    link,
			Snippet: firstNonEmptyString(r.Snippet, r.Text, r.Description),
		})
	}
	return results
}
