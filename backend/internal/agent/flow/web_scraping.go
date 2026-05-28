package flow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// ExecuteWebScraping fetches a URL and extracts text content.
func ExecuteWebScraping(ctx context.Context, fc *Context, config map[string]any) (string, error) {
	rawURL, _ := config["url"].(string)
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	rawURL = Interpolate(rawURL, fc.Variables)
	if err := CheckURL(rawURL, fc.AllowPrivateIPs); err != nil {
		return "", err
	}
	fc.Emit("Scraping " + rawURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AnythingLLM-AgentFlow/1.0")
	resp, err := fc.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("scrape: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		return string(body), nil // non-HTML: return raw
	}
	text := extractMainText(body)
	return text, nil
}

// extractMainText pulls readable text from HTML, preferring <article>,
// falling back to <main>, then <body>.
func extractMainText(body []byte) string {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return string(body)
	}

	// Try <article>
	if n := findNode(doc, "article"); n != nil {
		return strings.TrimSpace(renderText(n))
	}
	// Try <main>
	if n := findNode(doc, "main"); n != nil {
		return strings.TrimSpace(renderText(n))
	}
	// Fallback <body>
	if n := findNode(doc, "body"); n != nil {
		return strings.TrimSpace(renderText(n))
	}
	return strings.TrimSpace(renderText(doc))
}

func findNode(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNode(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func renderText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type == html.ElementNode {
		// Skip script/style
		if n.Data == "script" || n.Data == "style" || n.Data == "noscript" {
			return ""
		}
		var parts []string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if s := renderText(c); s != "" {
				parts = append(parts, s)
			}
		}
		out := strings.Join(parts, "")
		// Add spacing after block elements
		switch n.Data {
		case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "br", "article", "main", "section":
			out += "\n"
		}
		return out
	}
	var parts []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if s := renderText(c); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "")
}
