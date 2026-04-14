// Package image provides an image_generate tool backed by OpenAI-
// compatible /v1/images/generations endpoints.
package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/odysseythink/hermind/tool"
)

// Client talks to an OpenAI-compatible images endpoint.
type Client struct {
	BaseURL   string // default https://api.openai.com
	APIKey    string
	Model     string // default dall-e-3
	SaveDir   string // where b64 responses get written
	httpClient *http.Client
}

func NewClient(baseURL, apiKey, model, saveDir string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if model == "" {
		model = "dall-e-3"
	}
	if saveDir == "" {
		home, _ := os.UserHomeDir()
		saveDir = filepath.Join(home, ".hermes", "cache", "images")
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		Model:      model,
		SaveDir:    saveDir,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type genRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format"`
}

type genResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json,omitempty"`
		URL     string `json:"url,omitempty"`
	} `json:"data"`
}

// Generate calls /v1/images/generations and returns the local file
// paths of saved b64 results plus any URL results.
func (c *Client) Generate(ctx context.Context, prompt, size string) ([]string, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("image: api key required")
	}
	if err := os.MkdirAll(c.SaveDir, 0o755); err != nil {
		return nil, err
	}
	reqBody, _ := json.Marshal(genRequest{
		Model:          c.Model,
		Prompt:         prompt,
		N:              1,
		Size:           size,
		ResponseFormat: "b64_json",
	})
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/images/generations", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("image: status %d: %s", resp.StatusCode, string(body))
	}
	var out genResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	var paths []string
	ts := time.Now().UTC().Format("20060102-150405")
	for i, d := range out.Data {
		if d.B64JSON != "" {
			raw, err := base64.StdEncoding.DecodeString(d.B64JSON)
			if err != nil {
				return nil, err
			}
			path := filepath.Join(c.SaveDir, fmt.Sprintf("img-%s-%d.png", ts, i))
			if err := os.WriteFile(path, raw, 0o644); err != nil {
				return nil, err
			}
			paths = append(paths, path)
		} else if d.URL != "" {
			paths = append(paths, d.URL)
		}
	}
	return paths, nil
}

// Register adds the image_generate tool to reg. Only registers when
// cfg.APIKey is non-empty.
func Register(reg *tool.Registry, c *Client) {
	if c == nil || c.APIKey == "" {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "image_generate",
		Toolset:     "image",
		Description: "Generate an image from a text prompt and return its path.",
		Emoji:       "🖼",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "image_generate",
				Description: "Generate an image via DALL-E / OpenAI-compatible image API.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "prompt":{"type":"string"},
    "size":{"type":"string","description":"e.g. 1024x1024"}
  },
  "required":["prompt"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Prompt string `json:"prompt"`
				Size   string `json:"size,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Prompt) == "" {
				return tool.ToolError("prompt is required"), nil
			}
			paths, err := c.Generate(ctx, args.Prompt, args.Size)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"paths": paths}), nil
		},
	})
}
