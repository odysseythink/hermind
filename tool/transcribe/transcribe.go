// Package transcribe provides a transcribe_audio tool that sends a
// local audio file to an OpenAI-compatible Whisper endpoint and
// returns the recognized text.
package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/odysseythink/hermind/tool"
)

// Client talks to /v1/audio/transcriptions on the configured base URL.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string // default whisper-1
	http    *http.Client
}

func NewClient(baseURL, apiKey, model string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if model == "" {
		model = "whisper-1"
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		http:    &http.Client{Timeout: 180 * time.Second},
	}
}

// Transcribe uploads path to the Whisper endpoint and returns the
// recognized text.
func (c *Client) Transcribe(ctx context.Context, path string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("transcribe: api key required")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", err
	}
	_ = mw.WriteField("model", c.Model)
	_ = mw.WriteField("response_format", "json")
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcribe: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("transcribe: status %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

// Register adds the transcribe_audio tool.
func Register(reg *tool.Registry, c *Client) {
	if c == nil || c.APIKey == "" {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "transcribe_audio",
		Toolset:     "audio",
		Description: "Transcribe a local audio file to text via Whisper.",
		Emoji:       "🎙",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "transcribe_audio",
				Description: "Transcribe a local audio file path via a Whisper-compatible API.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"path":{"type":"string"}},
  "required":["path"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ Path string `json:"path"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Path) == "" {
				return tool.ToolError("path is required"), nil
			}
			text, err := c.Transcribe(ctx, args.Path)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]string{"text": text}), nil
		},
	})
}
