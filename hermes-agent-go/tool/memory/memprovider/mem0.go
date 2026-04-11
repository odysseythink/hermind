package memprovider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

// Mem0 is a Provider backed by the Mem0 cloud memory service.
type Mem0 struct {
	cfg       config.Mem0Config
	sessionID string
}

// NewMem0 builds a Mem0 provider with sensible defaults.
func NewMem0(cfg config.Mem0Config) *Mem0 {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.mem0.ai"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.UserID == "" {
		cfg.UserID = "default"
	}
	return &Mem0{cfg: cfg}
}

func (m *Mem0) Name() string { return "mem0" }

func (m *Mem0) Initialize(ctx context.Context, sessionID string) error {
	m.sessionID = sessionID
	return nil
}

func (m *Mem0) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	url := m.cfg.BaseURL + "/v1/memories/"
	body := map[string]any{
		"user_id": m.cfg.UserID,
		"messages": []map[string]string{
			{"role": "user", "content": userMsg},
			{"role": "assistant", "content": assistantMsg},
		},
	}
	return httpJSON(ctx, "POST", url, m.cfg.APIKey, body, nil)
}

func (m *Mem0) Shutdown(ctx context.Context) error { return nil }

type mem0SearchItem struct {
	Memory string `json:"memory"`
}

func (m *Mem0) search(ctx context.Context, query string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := m.cfg.BaseURL + "/v1/memories/search/"
	body := map[string]any{"query": query, "user_id": m.cfg.UserID, "limit": limit}
	var resp []mem0SearchItem
	if err := httpJSON(ctx, "POST", url, m.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp))
	for _, r := range resp {
		out = append(out, r.Memory)
	}
	return out, nil
}

func (m *Mem0) add(ctx context.Context, content string) error {
	url := m.cfg.BaseURL + "/v1/memories/"
	body := map[string]any{
		"user_id": m.cfg.UserID,
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
	}
	return httpJSON(ctx, "POST", url, m.cfg.APIKey, body, nil)
}

// RegisterTools registers mem0_recall and mem0_remember into reg.
func (m *Mem0) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "mem0_remember",
		Toolset:     "memory",
		Description: "Store a fact in Mem0 (external memory provider).",
		Emoji:       "💾",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "mem0_remember",
				Description: "Store a fact in the Mem0 memory store.",
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
			if err := m.add(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "mem0_recall",
		Toolset:     "memory",
		Description: "Recall memories from Mem0 by semantic query.",
		Emoji:       "🧩",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "mem0_recall",
				Description: "Search Mem0 memories and return matching content.",
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
			results, err := m.search(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
