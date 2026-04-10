// tool/web/search.go
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
)

const (
	exaDefaultURL = "https://api.exa.ai/search"
	exaTimeout    = 30 * time.Second
)

const webSearchSchema = `{
  "type": "object",
  "properties": {
    "query":       { "type": "string", "description": "Search query" },
    "num_results": { "type": "number", "description": "Number of results to return (default 5, max 20)" }
  },
  "required": ["query"]
}`

type webSearchArgs struct {
	Query      string `json:"query"`
	NumResults int    `json:"num_results,omitempty"`
}

type exaSearchRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
}

type exaSearchResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Text          string  `json:"text,omitempty"`
	PublishedDate string  `json:"publishedDate,omitempty"`
	Author        string  `json:"author,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

type webSearchResult struct {
	Query   string      `json:"query"`
	Results []exaResult `json:"results"`
}

// newWebSearchHandler builds a handler with an injected API key and endpoint.
// The CLI wires the real key from config. Tests can inject a custom URL.
func newWebSearchHandler(apiKey, endpoint string) tool.Handler {
	if endpoint == "" {
		endpoint = exaDefaultURL
	}
	client := &http.Client{Timeout: exaTimeout}
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args webSearchArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Query == "" {
			return tool.ToolError("query is required"), nil
		}
		if args.NumResults <= 0 {
			args.NumResults = 5
		}
		if args.NumResults > 20 {
			args.NumResults = 20
		}

		// Apply key fallback: explicit argument > injected > env var.
		key := apiKey
		if key == "" {
			key = os.Getenv("EXA_API_KEY")
		}
		if key == "" {
			return tool.ToolError("EXA_API_KEY not set"), nil
		}

		body, _ := json.Marshal(exaSearchRequest{
			Query:      args.Query,
			NumResults: args.NumResults,
		})

		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return tool.ToolError("new request: " + err.Error()), nil
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", key)
		httpReq.Header.Set("Accept", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			return tool.ToolError("exa request failed: " + err.Error()), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return tool.ToolError(fmt.Sprintf("exa http %d", resp.StatusCode)), nil
		}

		var out exaSearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return tool.ToolError("decode: " + err.Error()), nil
		}

		return tool.ToolResult(webSearchResult{
			Query:   args.Query,
			Results: out.Results,
		}), nil
	}
}
