// Package tts provides a speak tool that calls an OpenAI-compatible
// text-to-speech endpoint (/v1/audio/speech) and saves the MP3
// output to disk.
package tts

import (
	"bytes"
	"context"
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

// Client talks to /v1/audio/speech on the configured base URL.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string // default tts-1
	Voice   string // default alloy
	SaveDir string
	http    *http.Client
}

func NewClient(baseURL, apiKey, model, voice, saveDir string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if model == "" {
		model = "tts-1"
	}
	if voice == "" {
		voice = "alloy"
	}
	if saveDir == "" {
		home, _ := os.UserHomeDir()
		saveDir = filepath.Join(home, ".hermind", "cache", "audio")
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Voice:   voice,
		SaveDir: saveDir,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

type speakRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
	// ResponseFormat selects the audio container. mp3 is the widest
	// supported out of the box.
	ResponseFormat string `json:"response_format"`
}

// Speak synthesizes text to speech and writes an MP3 file under
// SaveDir. Returns the file path.
func (c *Client) Speak(ctx context.Context, text, voice string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("tts: api key required")
	}
	if voice == "" {
		voice = c.Voice
	}
	if err := os.MkdirAll(c.SaveDir, 0o755); err != nil {
		return "", err
	}
	body, _ := json.Marshal(speakRequest{
		Model:          c.Model,
		Input:          text,
		Voice:          voice,
		ResponseFormat: "mp3",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("tts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("tts: status %d: %s", resp.StatusCode, string(errBody))
	}
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	path := filepath.Join(c.SaveDir, fmt.Sprintf("speech-%s.mp3", time.Now().UTC().Format("20060102-150405.000")))
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Register adds the speak tool if the client has an API key.
func Register(reg *tool.Registry, c *Client) {
	if c == nil || c.APIKey == "" {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "speak",
		Toolset:     "audio",
		Description: "Synthesize speech from text and save an MP3.",
		Emoji:       "🔊",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "speak",
				Description: "Synthesize speech from text. Returns the path to the saved MP3.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "text":{"type":"string"},
    "voice":{"type":"string","description":"optional voice override"}
  },
  "required":["text"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Text  string `json:"text"`
				Voice string `json:"voice,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Text) == "" {
				return tool.ToolError("text is required"), nil
			}
			path, err := c.Speak(ctx, args.Text, args.Voice)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]string{"path": path}), nil
		},
	})
}
