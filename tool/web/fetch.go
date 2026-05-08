// tool/web/fetch.go
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/tool"
)

const (
	maxFetchBytes = 2 * 1024 * 1024 // 2 MiB
	fetchTimeout  = 30 * time.Second
)

const webFetchSchema = `{
  "type": "object",
  "properties": {
    "url":     { "type": "string", "description": "Absolute URL to fetch (http:// or https://)" },
    "method":  { "type": "string", "enum": ["GET","POST"], "description": "HTTP method (default GET)" },
    "headers": { "type": "object", "description": "Optional HTTP headers" },
    "body":    { "type": "string", "description": "Optional request body (for POST)" }
  },
  "required": ["url"]
}`

type webFetchArgs struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type webFetchResult struct {
	URL        string            `json:"url"`
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	Content    string            `json:"content"`
	Truncated  bool              `json:"truncated,omitempty"`
	ContentLen int               `json:"content_length"`
}

func webFetchHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args webFetchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.URL == "" {
		return tool.ToolError("url is required"), nil
	}

	method := args.Method
	if method == "" {
		method = "GET"
	}
	if method != "GET" && method != "POST" {
		return tool.ToolError("method must be GET or POST"), nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	var body io.Reader
	if args.Body != "" {
		body = stringReader(args.Body)
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, method, args.URL, body)
	if err != nil {
		return tool.ToolError("invalid URL: " + err.Error()), nil
	}
	for k, v := range args.Headers {
		httpReq.Header.Set(k, v)
	}
	// Default User-Agent
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "hermind/1.0")
	}

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return tool.ToolError("fetch failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	// Read up to maxFetchBytes, flag if truncated
	limited := io.LimitReader(resp.Body, int64(maxFetchBytes+1))
	raw2, err := io.ReadAll(limited)
	if err != nil {
		return tool.ToolError("read body: " + err.Error()), nil
	}
	truncated := false
	if len(raw2) > maxFetchBytes {
		raw2 = raw2[:maxFetchBytes]
		truncated = true
	}

	// Flatten headers
	hdr := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			hdr[k] = v[0]
		}
	}

	return tool.ToolResult(webFetchResult{
		URL:        args.URL,
		Status:     resp.StatusCode,
		Headers:    hdr,
		Content:    string(raw2),
		Truncated:  truncated,
		ContentLen: len(raw2),
	}), nil
}

// stringReader is a tiny helper to avoid importing strings for just one thing.
func stringReader(s string) io.Reader {
	return &strReader{data: []byte(s)}
}

type strReader struct {
	data []byte
	off  int
}

func (r *strReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}
