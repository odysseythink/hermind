// tool/web/scrape_site.go
package web

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"time"

	"github.com/odysseythink/hermind/tool"
)

// isPrivateHost returns true if the host is localhost, a private IP,
// or a link-local address.
func isPrivateHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// Try to strip port and re-parse
		if h, _, err := net.SplitHostPort(host); err == nil {
			ip = net.ParseIP(h)
			host = h
		}
	}
	if ip == nil {
		return false // hostname (e.g., example.com) — allow
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return false
}

const webScrapeSiteSchema = `{
  "type": "object",
  "properties": {
    "url":         { "type": "string", "description": "Starting URL to scrape (http:// or https://)" },
    "depth":       { "type": "integer", "description": "How many link-hops deep to crawl (default 1, max 3)", "minimum": 1, "maximum": 3 },
    "max_links":   { "type": "integer", "description": "Maximum total pages to scrape (default 10, max 50)", "minimum": 1, "maximum": 50 },
    "same_domain": { "type": "boolean", "description": "Only follow links on the same domain as the start URL (default true)" },
    "format":      { "type": "string", "enum": ["text","markdown"], "description": "Output format (default text)" }
  },
  "required": ["url"]
}`

type webScrapeSiteArgs struct {
	URL        string `json:"url"`
	Depth      int    `json:"depth,omitempty"`
	MaxLinks   int    `json:"max_links,omitempty"`
	SameDomain *bool  `json:"same_domain,omitempty"`
	Format     string `json:"format,omitempty"`
}

type scrapedPage struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type webScrapeSiteResult struct {
	URL          string        `json:"url"`
	PagesScraped int           `json:"pages_scraped"`
	PagesSkipped int           `json:"pages_skipped,omitempty"`
	Pages        []scrapedPage `json:"pages"`
}

func webScrapeSiteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args webScrapeSiteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}

	if args.URL == "" {
		return tool.ToolError("url is required"), nil
	}

	parsedURL, err := url.Parse(args.URL)
	if err != nil {
		return tool.ToolError("invalid URL: " + err.Error()), nil
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return tool.ToolError("url scheme must be http or https"), nil
	}
	if isPrivateHost(parsedURL.Host) {
		return tool.ToolError("url points to a private or internal address"), nil
	}

	// Apply defaults
	if args.Depth <= 0 {
		args.Depth = 1
	}
	if args.Depth > 3 {
		args.Depth = 3
	}
	if args.MaxLinks <= 0 {
		args.MaxLinks = 10
	}
	if args.MaxLinks > 50 {
		args.MaxLinks = 50
	}
	if args.Format == "" {
		args.Format = "text"
	}
	if args.Format != "text" && args.Format != "markdown" {
		return tool.ToolError("format must be text or markdown"), nil
	}

	sameDomain := true
	if args.SameDomain != nil {
		sameDomain = *args.SameDomain
	}

	overallTimeout := time.Duration(args.Depth*args.MaxLinks) * scrapePageTimeout
	if overallTimeout > 5*time.Minute {
		overallTimeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, overallTimeout)
	defer cancel()

	browser, cleanup, err := newBrowser(ctx)
	if err != nil {
		return tool.ToolError("failed to launch browser: " + err.Error()), nil
	}
	defer cleanup()

	var pages []scrapedPage
	skipped := 0

	discovered := map[string]bool{args.URL: true}
	queue := []string{args.URL}

	for d := 0; d < args.Depth; d++ {
		var nextQueue []string
		for _, u := range queue {
			if len(pages) >= args.MaxLinks {
				break
			}

			content, links, err := scrapePage(ctx, browser, u, args.Format)
			if err != nil {
				skipped++
				continue
			}

			pages = append(pages, scrapedPage{
				URL:     content.URL,
				Title:   content.Title,
				Content: content.Content,
			})

			if d+1 < args.Depth && len(pages) < args.MaxLinks {
				for _, link := range links {
					linkParsed, err := url.Parse(link)
					if err != nil {
						continue
					}
					// Normalize: strip fragment for deduplication
					linkParsed.Fragment = ""
					linkParsed.RawFragment = ""
					normalized := linkParsed.String()

					if isPrivateHost(linkParsed.Host) {
						continue
					}
					if sameDomain && linkParsed.Host != parsedURL.Host {
						continue
					}
					if !discovered[normalized] {
						discovered[normalized] = true
						nextQueue = append(nextQueue, normalized)
					}
				}
			}
		}
		queue = nextQueue
		if len(queue) == 0 {
			break
		}
	}

	return tool.ToolResult(webScrapeSiteResult{
		URL:          args.URL,
		PagesScraped: len(pages),
		PagesSkipped: skipped,
		Pages:        pages,
	}), nil
}
