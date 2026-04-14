// tool/web/extract.go
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/odysseythink/hermind/tool"
)

const (
	firecrawlDefaultURL = "https://api.firecrawl.dev/v1/scrape"
	firecrawlTimeout    = 60 * time.Second
)

const webExtractSchema = `{
  "type": "object",
  "properties": {
    "url":    { "type": "string", "description": "Absolute URL to extract content from" },
    "format": { "type": "string", "enum": ["markdown","html","text"], "description": "Output format (default markdown)" }
  },
  "required": ["url"]
}`

type webExtractArgs struct {
	URL    string `json:"url"`
	Format string `json:"format,omitempty"`
}

type firecrawlRequest struct {
	URL     string   `json:"url"`
	Formats []string `json:"formats"`
}

type firecrawlResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Markdown string         `json:"markdown,omitempty"`
		HTML     string         `json:"html,omitempty"`
		Text     string         `json:"text,omitempty"`
		Metadata map[string]any `json:"metadata,omitempty"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

type webExtractResult struct {
	URL      string         `json:"url"`
	Format   string         `json:"format"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// newWebExtractHandler builds a handler with injected API key and endpoint.
func newWebExtractHandler(apiKey, endpoint string) tool.Handler {
	if endpoint == "" {
		endpoint = firecrawlDefaultURL
	}
	client := &http.Client{Timeout: firecrawlTimeout}
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args webExtractArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.URL == "" {
			return tool.ToolError("url is required"), nil
		}
		format := args.Format
		if format == "" {
			format = "markdown"
		}
		if format != "markdown" && format != "html" && format != "text" {
			return tool.ToolError("format must be markdown, html, or text"), nil
		}

		key := apiKey
		if key == "" {
			key = os.Getenv("FIRECRAWL_API_KEY")
		}
		if key == "" {
			return tool.ToolError("FIRECRAWL_API_KEY not set"), nil
		}

		body, _ := json.Marshal(firecrawlRequest{
			URL:     args.URL,
			Formats: []string{format},
		})

		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return tool.ToolError("new request: " + err.Error()), nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+key)
		httpReq.Header.Set("Accept", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			return tool.ToolError("firecrawl request failed: " + err.Error()), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return tool.ToolError(fmt.Sprintf("firecrawl http %d", resp.StatusCode)), nil
		}

		var out firecrawlResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return tool.ToolError("decode: " + err.Error()), nil
		}
		if !out.Success {
			return tool.ToolError("firecrawl: " + out.Error), nil
		}

		var content string
		switch format {
		case "markdown":
			content = out.Data.Markdown
		case "html":
			content = out.Data.HTML
		case "text":
			content = out.Data.Text
		}

		return tool.ToolResult(webExtractResult{
			URL:      args.URL,
			Format:   format,
			Content:  content,
			Metadata: out.Data.Metadata,
		}), nil
	}
}
