package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestSerpApi_Google(t *testing.T) {
	fixture := `{
		"knowledge_graph": {
			"title": "Go",
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

	results := runSerpapiNormalize(t, "google", fixture)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	if results[0].Title != "Go" || results[0].Snippet != "Go is a programming language." {
		t.Errorf("knowledge graph mismatch: %+v", results[0])
	}
	if results[1].Title != "Answer Box" || results[1].Snippet != "Go was designed at Google." {
		t.Errorf("answer box mismatch: %+v", results[1])
	}
	if results[2].Title != "The Go Programming Language" {
		t.Errorf("organic title mismatch: %+v", results[2])
	}
}

func TestSerpApi_Baidu(t *testing.T) {
	fixture := `{
		"answer_box": {
			"answer": "Baidu answer"
		},
		"organic_results": [
			{
				"title": "Go 语言",
				"link": "https://golang.google.cn/",
				"snippet": "Go 是 Google 的编程语言。"
			}
		]
	}`

	results := runSerpapiNormalize(t, "baidu", fixture)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Snippet != "Baidu answer" {
		t.Errorf("answer box mismatch: %+v", results[0])
	}
	if results[1].Title != "Go 语言" {
		t.Errorf("organic title mismatch: %+v", results[1])
	}
}

func TestSerpApi_Amazon(t *testing.T) {
	fixture := `{
		"organic_results": [
			{
				"title": "Go in Action",
				"link": "https://www.amazon.com/dp/1617291781",
				"snippet": "A book about Go."
			},
			{
				"title": "",
				"link": "https://www.amazon.com/dp/invalid",
				"snippet": "Skipped"
			}
		]
	}`

	results := runSerpapiNormalize(t, "amazon", fixture)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Go in Action" {
		t.Errorf("title mismatch: %+v", results[0])
	}
}

func TestSerpApi_MissingKey(t *testing.T) {
	provider := getSearchProvider("serpapi")
	if provider == nil {
		t.Fatal("serpapi provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "SerpApi API key not configured. Set AgentSerpApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestSerpApi_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live SerpApi search")
	}

	apiKey := os.Getenv("AGENT_SERPAPI_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_SERPAPI_API_KEY to run live SerpApi search")
	}

	provider := getSearchProvider("serpapi")
	if provider == nil {
		t.Fatal("serpapi provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentSerpApiKey":    apiKey,
		"AgentSerpApiEngine": "google",
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

func runSerpapiNormalize(t *testing.T, engine, payload string) []SearchResult {
	t.Helper()
	var resp serpapiResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return normalizeSerpapiResponse(engine, &resp)
}
