package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// WebsiteDepthRequest is the payload for the website-depth extension.
type WebsiteDepthRequest struct {
	URL      string `json:"url"`
	Depth    int    `json:"depth"`
	MaxLinks int    `json:"maxLinks"`
}

// PageInfo holds scraped content for a single page.
type PageInfo struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// WebsiteDepthExtension implements BFS crawling for websites.
type WebsiteDepthExtension struct {
	httpClient *http.Client
}

// NewWebsiteDepthExtension creates a new WebsiteDepthExtension.
func NewWebsiteDepthExtension() *WebsiteDepthExtension {
	return &WebsiteDepthExtension{httpClient: &http.Client{}}
}

// NewWebsiteDepthExtensionWithClient creates a new WebsiteDepthExtension with a custom HTTP client.
func NewWebsiteDepthExtensionWithClient(client *http.Client) *WebsiteDepthExtension {
	return &WebsiteDepthExtension{httpClient: client}
}

// Name returns the extension name.
func (w *WebsiteDepthExtension) Name() string { return "website-depth" }

// Handle routes website-depth extension requests.
func (w *WebsiteDepthExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}
	if endpoint != "/ext/website-depth" {
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
	return w.crawl(ctx, body)
}

func (w *WebsiteDepthExtension) crawl(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req WebsiteDepthRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	if req.Depth <= 0 {
		req.Depth = 1
	}
	if req.MaxLinks <= 0 {
		req.MaxLinks = 20
	}

	startURL, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if startURL.Scheme == "" {
		startURL.Scheme = "https"
	}

	visited := make(map[string]bool)
	var pages []PageInfo
	queue := []struct {
		url   string
		depth int
	}{{url: startURL.String(), depth: 0}}

	for len(queue) > 0 && len(pages) < req.MaxLinks {
		item := queue[0]
		queue = queue[1:]

		if visited[item.url] {
			continue
		}
		visited[item.url] = true

		page, links, err := w.fetchPage(ctx, item.url)
		if err != nil {
			continue
		}
		pages = append(pages, *page)

		if item.depth+1 < req.Depth {
			for _, link := range links {
				if len(pages)+len(queue) >= req.MaxLinks {
					break
				}
				if !visited[link] {
					queue = append(queue, struct {
						url   string
						depth int
					}{url: link, depth: item.depth + 1})
				}
			}
		}
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"pages": pages},
	}, nil
}

func (w *WebsiteDepthExtension) fetchPage(ctx context.Context, pageURL string) (*PageInfo, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	title := doc.Find("title").First().Text()
	doc.Find("script, style, noscript").Remove()
	text := doc.Find("body").Text()
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	var links []string
	base, _ := url.Parse(pageURL)
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		u, err := base.Parse(href)
		if err != nil {
			return
		}
		u.Fragment = ""
		u.RawQuery = ""
		if u.Host == base.Host && !visitedLink(links, u.String()) {
			links = append(links, u.String())
		}
	})

	return &PageInfo{
		URL:     pageURL,
		Title:   strings.TrimSpace(title),
		Content: strings.Join(cleaned, "\n"),
	}, links, nil
}

func visitedLink(links []string, target string) bool {
	for _, l := range links {
		if l == target {
			return true
		}
	}
	return false
}
