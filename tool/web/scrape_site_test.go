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
		{"invalid json", `{"bad json"`, "invalid arguments"},
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
		t.Errorf("missing pages_scraped in JSON: %s", s)
	}
	if !strings.Contains(s, "pages_skipped") {
		t.Errorf("missing pages_skipped in JSON: %s", s)
	}
	if !strings.Contains(s, "hello world") {
		t.Errorf("missing page content in JSON: %s", s)
	}
}

func TestWebScrapeSiteResult_OmitEmpty(t *testing.T) {
	res := webScrapeSiteResult{
		URL:          "https://example.com",
		PagesScraped: 1,
		PagesSkipped: 0,
		Pages:        []scrapedPage{{URL: "https://example.com/", Title: "Home", Content: "hello"}},
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	// PagesSkipped is 0 and has omitempty, so it should NOT appear
	if strings.Contains(s, "pages_skipped") {
		t.Errorf("pages_skipped should be omitted when 0, but found in JSON: %s", s)
	}
}

func TestScrapedPage_Marshal(t *testing.T) {
	p := scrapedPage{URL: "https://x.com", Title: "X", Content: "content"}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"url"`) {
		t.Errorf("missing url key in JSON: %s", string(b))
	}
}
