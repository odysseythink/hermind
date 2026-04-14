package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

// Supermemory is a Provider backed by the Supermemory cloud API.
type Supermemory struct {
	cfg       config.SupermemoryConfig
	sessionID string
}

func NewSupermemory(cfg config.SupermemoryConfig) *Supermemory {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.supermemory.ai"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.UserID == "" {
		cfg.UserID = "default"
	}
	return &Supermemory{cfg: cfg}
}

func (s *Supermemory) Name() string { return "supermemory" }

func (s *Supermemory) Initialize(ctx context.Context, sessionID string) error {
	s.sessionID = sessionID
	return nil
}

func (s *Supermemory) add(ctx context.Context, content string) error {
	url := s.cfg.BaseURL + "/v3/memories"
	body := map[string]any{
		"content":        content,
		"user_id":        s.cfg.UserID,
		"container_tags": []string{"hermes"},
	}
	return httpJSON(ctx, "POST", url, s.cfg.APIKey, body, nil)
}

func (s *Supermemory) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return s.add(ctx, fmt.Sprintf("user: %s\nassistant: %s", userMsg, assistantMsg))
}

func (s *Supermemory) Shutdown(ctx context.Context) error { return nil }

type supermemorySearchResponse struct {
	Results []struct {
		Content string `json:"content"`
	} `json:"results"`
}

func (s *Supermemory) search(ctx context.Context, q string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := s.cfg.BaseURL + "/v3/search"
	body := map[string]any{"q": q, "user_id": s.cfg.UserID, "limit": limit}
	var resp supermemorySearchResponse
	if err := httpJSON(ctx, "POST", url, s.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Content)
	}
	return out, nil
}

// RegisterTools registers supermemory_remember and supermemory_recall.
func (s *Supermemory) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "supermemory_remember",
		Toolset:     "memory",
		Description: "Store a fact in Supermemory (external memory provider).",
		Emoji:       "🧠",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "supermemory_remember",
				Description: "Store a fact in Supermemory for future recall.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"content":{"type":"string","description":"Text to remember"}},
  "required":["content"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := s.add(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "supermemory_recall",
		Toolset:     "memory",
		Description: "Recall memories from Supermemory by query.",
		Emoji:       "🔭",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "supermemory_recall",
				Description: "Search Supermemory and return matching content.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "query":{"type":"string","description":"Search query"},
    "limit":{"type":"number","description":"Max results (default 5)"}
  },
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Query) == "" {
				return tool.ToolError("query is required"), nil
			}
			results, err := s.search(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
