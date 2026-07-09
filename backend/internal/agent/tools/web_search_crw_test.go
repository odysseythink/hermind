package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestCrw_ParseResponse(t *testing.T) {
	fixture := `{
		"success": true,
		"data": [
			{
				"title": "The Go Programming Language",
				"url": "https://golang.org/",
				"description": "Go is an open source programming language."
			},
			{
				"title": "Go Packages",
				"url": "https://pkg.go.dev/",
				"description": "Find, add, and publish Go packages."
			}
		]
	}`

	var resp crwResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := normalizeCrwResults(resp.Data)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "The Go Programming Language" {
		t.Errorf("title mismatch: %+v", results[0])
	}
}

func TestCrw_ErrorResponse(t *testing.T) {
	fixture := `{"success": false, "error": "rate limit exceeded"}`
	var resp crwResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if resp.Success == nil || *resp.Success {
		t.Fatal("expected success=false")
	}
	if resp.Error != "rate limit exceeded" {
		t.Fatalf("error mismatch: %q", resp.Error)
	}
}

func TestCrw_MissingKey(t *testing.T) {
	provider := getSearchProvider("crw-search")
	if provider == nil {
		t.Fatal("crw-search provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "fastCRW API key not configured. Set AgentCrwApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestCrw_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live fastCRW search")
	}

	apiKey := os.Getenv("AGENT_CRW_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_CRW_API_KEY to run live fastCRW search")
	}

	provider := getSearchProvider("crw-search")
	if provider == nil {
		t.Fatal("crw-search provider not registered")
	}

	settings := map[string]string{
		"AgentCrwApiKey": apiKey,
	}
	if url := os.Getenv("AGENT_CRW_API_URL"); url != "" {
		settings["AgentCrwApiUrl"] = url
	}

	results, err := provider.Search(context.Background(), "golang", settings, &config.Config{})
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

func normalizeCrwResults(items []crwResult) []SearchResult {
	results := make([]SearchResult, 0, len(items))
	for _, r := range items {
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
