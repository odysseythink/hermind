package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestSearchApi_ParseResponse(t *testing.T) {
	fixture := `{
		"knowledge_graph": {
			"description": "Go is a programming language."
		},
		"answer_box": {
			"answer": "Go was designed at Google."
		},
		"organic_results": [
			{
				"title": "The Go Programming Language",
				"link": "https://golang.org/",
				"snippet": "Go is an open source programming language."
			},
			{
				"title": "Go Packages",
				"link": "https://pkg.go.dev/",
				"snippet": "Find, add, and publish Go packages."
			}
		]
	}`

	var resp searchapiResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := normalizeSearchapiResponse(&resp)

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	if results[0].Title != "Knowledge Graph" || results[0].Snippet != "Go is a programming language." {
		t.Errorf("knowledge graph mismatch: %+v", results[0])
	}
	if results[1].Title != "Answer Box" || results[1].Snippet != "Go was designed at Google." {
		t.Errorf("answer box mismatch: %+v", results[1])
	}
	if results[2].Title != "The Go Programming Language" {
		t.Errorf("organic title mismatch: %+v", results[2])
	}
}

func TestSearchApi_MissingKey(t *testing.T) {
	provider := getSearchProvider("searchapi")
	if provider == nil {
		t.Fatal("searchapi provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "SearchApi API key not configured. Set AgentSearchApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestSearchApi_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live SearchApi search")
	}

	apiKey := os.Getenv("AGENT_SEARCHAPI_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_SEARCHAPI_API_KEY to run live SearchApi search")
	}

	provider := getSearchProvider("searchapi")
	if provider == nil {
		t.Fatal("searchapi provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentSearchApiKey":    apiKey,
		"AgentSearchApiEngine": "google",
	}, &config.Config{})
	if err != nil {
		t.Fatalf("live search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one live result")
	}

	for i, r := range results {
		if r.Title == "" {
			t.Fatalf("result %d has empty title: %+v", i, r)
		}
	}
}
