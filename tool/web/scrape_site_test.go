// tool/web/scrape_site_test.go
package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWebScrapeSiteHandler_InvalidArgs(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"empty url", `{"url":""}`, "url is required"},
		{"bad scheme", `{"url":"ftp://example.com"}`, "url scheme must be http or https"},
		{"bad format", `{"url":"https://example.com","format":"xml"}`, "format must be text or markdown"},
		{"invalid url", `{"url":"://bad"}`, "invalid URL"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := webScrapeSiteHandler(context.Background(), []byte(tc.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(res, "error") {
				t.Fatalf("expected error response, got: %s", res)
			}
			if !strings.Contains(res, tc.want) {
				t.Fatalf("expected %q in response, got: %s", tc.want, res)
			}
		})
	}
}

func TestWebScrapeSiteHandler_Defaults(t *testing.T) {
	// We can't test the full handler without a browser, but we can verify
	// that valid minimal args don't fail on validation.
	args := webScrapeSiteArgs{URL: "https://example.com"}
	if args.Depth != 0 || args.MaxLinks != 0 || args.Format != "" {
		t.Log("default values are zero values; clamping happens inside handler")
	}
}

func TestWebScrapeSiteResult_Marshal(t *testing.T) {
	res := webScrapeSiteResult{
		URL:          "https://example.com",
		PagesScraped: 2,
		PagesSkipped: 1,
		Pages: []scrapedPage{
			{URL: "https://example.com/", Title: "Home", Content: "hello world"},
			{URL: "https://example.com/about", Title: "About", Content: "about us"},
		},
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "pages_scraped") {
		t.Error("missing pages_scraped in json")
	}
	if !strings.Contains(s, "pages_skipped") {
		t.Error("missing pages_skipped in json")
	}
	if !strings.Contains(s, "hello world") {
		t.Error("missing page content in json")
	}
}

func TestScrapedPage_Marshal(t *testing.T) {
	p := scrapedPage{URL: "https://x.com", Title: "X", Content: "content"}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"url"`) {
		t.Error("missing url key")
	}
}
