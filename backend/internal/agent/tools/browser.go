package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
	"golang.org/x/net/html"
)

const browserMaxBodyBytes = 1 << 20 // 1 MiB cap
const browserHTTPTimeout = 30 * time.Second
const browserCDPTimeout = 30 * time.Second

func NewWebScrapingSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "web-scraping",
		Toolset:        "web",
		Description:    "Fetch a URL and return its main textual content, or capture a screenshot. Supports dynamic JavaScript-rendered pages via headless browser.",
		MaxResultChars: 32 * 1024,
		Schema: core.ToolDefinition{
			Name:        "web-scraping",
			Description: "Fetch and extract web page content or capture a screenshot",
			Parameters:  browserSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				URL      string `json:"url"`
				Action   string `json:"action"`
				Selector string `json:"selector"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if args.URL == "" {
				return tool.Error("url is required"), nil
			}
			if args.Action == "" {
				args.Action = "scrape"
			}
			if args.Action != "scrape" && args.Action != "screenshot" {
				return tool.Error("action must be 'scrape' or 'screenshot'"), nil
			}

			u, err := url.Parse(args.URL)
			if err != nil {
				return tool.Error("invalid url"), nil
			}
			if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "data" {
				return tool.Error("only http/https/data URLs allowed"), nil
			}

			// SSRF guard for remote URLs
			if u.Scheme == "http" || u.Scheme == "https" {
				if err := flow.CheckURL(args.URL, false); err != nil {
					return tool.Error("url blocked: " + err.Error()), nil
				}
			}

			tc.Emit(fmt.Sprintf("Browsing %s (%s)", args.URL, args.Action))

			// Try chromedp first for http/https
			if u.Scheme == "http" || u.Scheme == "https" {
				result, err := tryChromedp(ctx, args.URL, args.Action, args.Selector)
				if err == nil {
					return result, nil
				}
				// Screenshot must not fallback to static fetch
				if args.Action == "screenshot" {
					return tool.Error("screenshot failed: " + err.Error()), nil
				}
				tc.Emit("Browser unavailable, falling back to static fetch")
			}

			// Static HTTP fallback (also handles data URLs directly)
			return staticFetch(ctx, args.URL)
		},
	}
}

func tryChromedp(ctx context.Context, pageURL, action, selector string) (string, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	deadline := time.Now().Add(browserCDPTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	runCtx, runCancel := context.WithDeadline(taskCtx, deadline)
	defer runCancel()

	var actions []chromedp.Action
	actions = append(actions, chromedp.Navigate(pageURL))

	waitSel := "body"
	if selector != "" {
		waitSel = selector
	}
	actions = append(actions, chromedp.WaitVisible(waitSel, chromedp.ByQuery))

	switch action {
	case "scrape":
		var text string
		if selector != "" {
			actions = append(actions, chromedp.Text(selector, &text, chromedp.ByQuery))
		} else {
			actions = append(actions, chromedp.Evaluate("document.body.innerText", &text))
		}
		if err := chromedp.Run(runCtx, actions...); err != nil {
			return "", err
		}
		return tool.Result(map[string]any{
			"url":     pageURL,
			"title":   "",
			"content": strings.TrimSpace(text),
		}), nil

	case "screenshot":
		var buf []byte
		if selector != "" {
			actions = append(actions, chromedp.Screenshot(selector, &buf, chromedp.ByQuery))
		} else {
			actions = append(actions, chromedp.FullScreenshot(&buf, 90))
		}
		if err := chromedp.Run(runCtx, actions...); err != nil {
			return "", err
		}
		return tool.Result(map[string]any{
			"url":               pageURL,
			"screenshot_base64": base64.StdEncoding.EncodeToString(buf),
			"mime_type":         "image/png",
		}), nil
	}

	return "", fmt.Errorf("unknown action: %s", action)
}

func staticFetch(ctx context.Context, pageURL string) (string, error) {
	client := &http.Client{Timeout: browserHTTPTimeout}
	req, _ := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return tool.Error("fetch: " + err.Error()), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return tool.Error(fmt.Sprintf("http %d", resp.StatusCode)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, browserMaxBodyBytes))
	if err != nil {
		return tool.Error("read body: " + err.Error()), nil
	}

	text, title := extractMainText(body)
	return tool.Result(map[string]any{
		"url":     pageURL,
		"title":   title,
		"content": text,
	}), nil
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

func browserSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to visit"},
			"action": {"type": "string", "enum": ["scrape", "screenshot"], "default": "scrape", "description": "Action to perform"},
			"selector": {"type": "string", "description": "CSS selector to target a specific element (optional)"}
		},
		"required": ["url"]
	}`))
}
