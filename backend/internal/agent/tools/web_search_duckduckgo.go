package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
)

// duckDuckGoProvider searches DuckDuckGo via their HTML endpoint.
// It requires no API key and is the default fallback provider.
type duckDuckGoProvider struct {
	client *http.Client
}

func init() {
	registerSearchProvider("duckduckgo-engine", &duckDuckGoProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	})
}

func (p *duckDuckGoProvider) Name() string { return "DuckDuckGo" }

func (p *duckDuckGoProvider) Search(ctx context.Context, query string, _ map[string]string, _ *config.Config) ([]SearchResult, error) {
	searchURL := "https://html.duckduckgo.com/html?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("duckduckgo: http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: read body: %w", err)
	}

	return parseDuckDuckGoResults(string(body)), nil
}

var (
	duckDuckGoResultSplit = `<div class="result results_links`
	duckDuckGoTitleRe     = regexp.MustCompile(`<a[^>]*class="result__a"[^>]*>(.*?)</a>`)
	duckDuckGoLinkRe      = regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]*)"`)
	duckDuckGoSnippetRe   = regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>(.*?)</a>`)
	duckDuckGoBoldRe      = regexp.MustCompile(`</?b>`)
)

func parseDuckDuckGoResults(html string) []SearchResult {
	segments := strings.Split(html, duckDuckGoResultSplit)
	if len(segments) <= 1 {
		return nil
	}

	results := make([]SearchResult, 0, len(segments)-1)
	for _, segment := range segments[1:] {
		titleMatch := duckDuckGoTitleRe.FindStringSubmatch(segment)
		linkMatch := duckDuckGoLinkRe.FindStringSubmatch(segment)
		snippetMatch := duckDuckGoSnippetRe.FindStringSubmatch(segment)

		if titleMatch == nil || linkMatch == nil || snippetMatch == nil {
			continue
		}

		title := strings.TrimSpace(stripHTMLTags(titleMatch[1]))
		link := strings.TrimSpace(extractDuckDuckGoURL(linkMatch[1]))
		snippet := strings.TrimSpace(duckDuckGoBoldRe.ReplaceAllString(snippetMatch[1], ""))

		if title == "" || link == "" || snippet == "" {
			continue
		}

		results = append(results, SearchResult{
			Title:   title,
			Link:    link,
			Snippet: snippet,
		})
	}

	return results
}

// extractDuckDuckGoURL decodes a DuckDuckGo redirect URL back to the destination URL.
// DDG links look like //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=...
func extractDuckDuckGoURL(ddgLink string) string {
	if ddgLink == "" {
		return ""
	}

	fullURL := ddgLink
	if strings.HasPrefix(ddgLink, "//") {
		fullURL = "https:" + ddgLink
	}

	u, err := url.Parse(fullURL)
	if err != nil {
		return ddgLink
	}

	actualURL := u.Query().Get("uddg")
	if actualURL == "" {
		return ddgLink
	}

	decoded, err := url.QueryUnescape(actualURL)
	if err != nil {
		return actualURL
	}
	return decoded
}

// stripHTMLTags removes simple HTML tags from a string.
func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}
