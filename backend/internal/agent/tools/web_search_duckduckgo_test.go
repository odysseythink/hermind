package tools

import (
	"context"
	"os"
	"testing"
)

func TestDuckDuckGo_ExtractURL(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "decode uddg param",
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=abc",
			expected: "https://example.com",
		},
		{
			name:     "already absolute https",
			input:    "https://duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org&rut=abc",
			expected: "https://golang.org",
		},
		{
			name:     "no uddg returns original",
			input:    "https://example.com/direct",
			expected: "https://example.com/direct",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid url returns original",
			input:    "://not-a-url",
			expected: "://not-a-url",
		},
		{
			name:     "uddg not double encoded",
			input:    "//duckduckgo.com/l/?uddg=https://pkg.go.dev",
			expected: "https://pkg.go.dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractDuckDuckGoURL(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestDuckDuckGo_ParseResults(t *testing.T) {
	fixture := `<!DOCTYPE html>
<html>
<body>
<div class="result results_links results_links_deep web-result">
	<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2Fdoc">The Go Programming Language</a>
	<a class="result__snippet">Go is an open source programming language that makes it <b>easy</b> to build simple, reliable, and efficient software.</a>
</div>
<div class="result results_links results_links_deep web-result">
	<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fpkg.go.dev">Go Packages</a>
	<a class="result__snippet">Find, add, and publish Go packages on pkg.go.dev.</a>
</div>
<div class="result results_links results_links_deep web-result">
	<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fmissing-snippet">Should Be Skipped</a>
</div>
</body>
</html>`

	results := parseDuckDuckGoResults(fixture)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	first := results[0]
	if first.Title != "The Go Programming Language" {
		t.Errorf("title: expected %q, got %q", "The Go Programming Language", first.Title)
	}
	if first.Link != "https://golang.org/doc" {
		t.Errorf("link: expected %q, got %q", "https://golang.org/doc", first.Link)
	}
	if first.Snippet != "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software." {
		t.Errorf("snippet: expected %q, got %q", "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.", first.Snippet)
	}

	second := results[1]
	if second.Title != "Go Packages" {
		t.Errorf("title: expected %q, got %q", "Go Packages", second.Title)
	}
	if second.Link != "https://pkg.go.dev" {
		t.Errorf("link: expected %q, got %q", "https://pkg.go.dev", second.Link)
	}
}

func TestDuckDuckGo_LiveSearch(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to run live DuckDuckGo search")
	}

	provider := getSearchProvider("duckduckgo-engine")
	if provider == nil {
		t.Fatal("duckduckgo-engine provider not registered")
	}

	results, err := provider.Search(context.Background(), "golang", nil, nil)
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
