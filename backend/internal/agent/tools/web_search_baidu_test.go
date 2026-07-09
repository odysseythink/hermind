package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
)

func TestBaidu_ParseResponse(t *testing.T) {
	fixture := `{
		"references": [
			{
				"title": "The Go Programming Language",
				"url": "https://golang.org/",
				"snippet": "Go is an open source programming language.",
				"type": "web"
			},
			{
				"title": "Go Packages",
				"url": "https://pkg.go.dev/",
				"content": "Find, add, and publish Go packages.",
				"resource_type": "web"
			},
			{
				"title": "Duplicate",
				"url": "https://golang.org/",
				"snippet": "Should be deduped",
				"type": "web"
			},
			{
				"title": "Video Result",
				"url": "https://video.example.com/",
				"snippet": "Should be filtered",
				"type": "video"
			}
		]
	}`

	var resp baiduResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := normalizeBaiduReferences(resp.References)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "The Go Programming Language" {
		t.Errorf("title mismatch: %+v", results[0])
	}
	if results[1].Snippet != "Find, add, and publish Go packages." {
		t.Errorf("snippet mismatch: %+v", results[1])
	}
}

func TestBaidu_MissingKey(t *testing.T) {
	provider := getSearchProvider("baidu-search")
	if provider == nil {
		t.Fatal("baidu-search provider not registered")
	}

	_, err := provider.Search(context.Background(), "golang", map[string]string{}, &config.Config{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if err.Error() != "Baidu Search API key not configured. Set AgentBaiduSearchApiKey in settings." {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestBaidu_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live Baidu search")
	}

	apiKey := os.Getenv("AGENT_BAIDU_SEARCH_API_KEY")
	if apiKey == "" {
		t.Skip("set AGENT_BAIDU_SEARCH_API_KEY to run live Baidu search")
	}

	provider := getSearchProvider("baidu-search")
	if provider == nil {
		t.Fatal("baidu-search provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", map[string]string{
		"AgentBaiduSearchApiKey": apiKey,
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
