package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const ddgDefaultURL = "https://html.duckduckgo.com/html/"

type ddgProvider struct {
	endpoint string
	client   *http.Client
}

func newDDGProvider(endpoint string) *ddgProvider {
	return &ddgProvider{
		endpoint: endpoint,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *ddgProvider) ID() string { return "ddg" }

// Configured returns true unconditionally: DuckDuckGo's HTML endpoint
// does not require an API key.
func (p *ddgProvider) Configured() bool { return true }

func (p *ddgProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error) {
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = ddgDefaultURL
	}
	form := url.Values{}
	form.Set("q", q)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "hermind/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	// Rate-limit detection: DDG renders an .anomaly-modal element
	// (and/or copy mentioning "anomaly") when throttling.
	if doc.Find(".anomaly-modal").Length() > 0 || strings.Contains(strings.ToLower(doc.Text()), "anomaly") {
		return nil, fmt.Errorf("rate limited")
	}

	results := make([]SearchResult, 0, n)
	doc.Find(".result").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		if len(results) >= n {
			return false
		}
		link := sel.Find(".result__a").First()
		title := strings.TrimSpace(link.Text())
		href, _ := link.Attr("href")
		actual := decodeDDGLink(href)
		if title == "" || actual == "" {
			return true
		}
		snippet := strings.TrimSpace(sel.Find(".result__snippet").First().Text())
		results = append(results, SearchResult{
			Title:   title,
			URL:     actual,
			Snippet: snippet,
		})
		return true
	})
	return results, nil
}

// decodeDDGLink extracts the real destination from DDG's /l/?uddg=...
// wrapper. If raw is already an absolute URL, it is returned as-is.
func decodeDDGLink(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if dest := u.Query().Get("uddg"); dest != "" {
		return dest
	}
	return ""
}
