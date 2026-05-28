package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
	"golang.org/x/net/html"
)

const wsMaxBodyBytes = 1 << 20 // 1 MiB cap
const wsHTTPTimeout = 30 * time.Second

func NewWebScrapingSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "web-scraping",
		Toolset:        "web",
		Description:    "Fetch a URL and return its main textual content (article > main > body).",
		MaxResultChars: 8 * 1024,
		Schema: core.ToolDefinition{
			Name:        "web-scraping",
			Description: "Fetch and extract web page content",
			Parameters:  webScrapingSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if args.URL == "" {
				return tool.Error("url is required"), nil
			}
			u, err := url.Parse(args.URL)
			if err != nil {
				return tool.Error("invalid url"), nil
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return tool.Error("only http/https URLs allowed"), nil
			}
			tc.Emit("Scraping " + args.URL)

			client := &http.Client{Timeout: wsHTTPTimeout}
			req, _ := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
			req.Header.Set("User-Agent", "Hermind-Agent/1.0")
			resp, err := client.Do(req)
			if err != nil {
				return tool.Error("fetch: " + err.Error()), nil
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				return tool.Error(fmt.Sprintf("http %d", resp.StatusCode)), nil
			}

			body, err := io.ReadAll(io.LimitReader(resp.Body, wsMaxBodyBytes))
			if err != nil {
				return tool.Error("read body: " + err.Error()), nil
			}

			text, title := extractMainText(body)
			return tool.Result(map[string]any{
				"url":     args.URL,
				"title":   title,
				"content": text,
			}), nil
		},
	}
}

func extractMainText(body []byte) (text, title string) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return string(body), ""
	}

	var findTitle func(*html.Node) string
	findTitle = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			return strings.TrimSpace(n.FirstChild.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := findTitle(c); t != "" {
				return t
			}
		}
		return ""
	}
	title = findTitle(doc)

	var findRoot func(*html.Node, string) *html.Node
	findRoot = func(n *html.Node, tag string) *html.Node {
		if n.Type == html.ElementNode && n.Data == tag {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if r := findRoot(c, tag); r != nil {
				return r
			}
		}
		return nil
	}

	root := findRoot(doc, "article")
	if root == nil {
		root = findRoot(doc, "main")
	}
	if root == nil {
		root = findRoot(doc, "body")
	}
	if root == nil {
		return "", title
	}

	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "aside", "noscript", "iframe":
				return
			}
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				sb.WriteString(t)
				sb.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.TrimSpace(sb.String()), title
}

func webScrapingSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"url": {"type": "string"}
		},
		"required": ["url"]
	}`))
}
