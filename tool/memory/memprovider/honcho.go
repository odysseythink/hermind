package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

// Honcho is a Provider backed by the Honcho memory service.
//
// It implements only the minimal subset needed for Plan 6c:
//   - append turns to the peer's message stream (/messages)
//   - search the peer's memories on demand (/search)
type Honcho struct {
	cfg       config.HonchoConfig
	sessionID string
}

// NewHoncho constructs a Honcho provider from configuration. The
// config is normalized: empty workspace defaults to "hermes", empty
// peer defaults to "me", empty base URL to the public demo endpoint.
func NewHoncho(cfg config.HonchoConfig) *Honcho {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://demo.honcho.dev"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Workspace == "" {
		cfg.Workspace = "hermind"
	}
	if cfg.Peer == "" {
		cfg.Peer = "me"
	}
	return &Honcho{cfg: cfg}
}

func (h *Honcho) Name() string { return "honcho" }

func (h *Honcho) Initialize(ctx context.Context, sessionID string) error {
	h.sessionID = sessionID
	return nil
}

func (h *Honcho) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return h.addMessage(ctx, fmt.Sprintf("user: %s\nassistant: %s", userMsg, assistantMsg))
}

func (h *Honcho) Shutdown(ctx context.Context) error { return nil }

// addMessage POSTs a single memory message to the peer.
func (h *Honcho) addMessage(ctx context.Context, content string) error {
	url := fmt.Sprintf("%s/v1/workspaces/%s/peers/%s/messages",
		h.cfg.BaseURL, h.cfg.Workspace, h.cfg.Peer)
	body := map[string]any{
		"content":    content,
		"session_id": h.sessionID,
	}
	return httpJSON(ctx, "POST", url, h.cfg.APIKey, body, nil)
}

type honchoSearchResponse struct {
	Results []struct {
		Content string `json:"content"`
	} `json:"results"`
}

// search queries the peer for memories matching q.
func (h *Honcho) search(ctx context.Context, q string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := fmt.Sprintf("%s/v1/workspaces/%s/peers/%s/search",
		h.cfg.BaseURL, h.cfg.Workspace, h.cfg.Peer)
	body := map[string]any{"query": q, "limit": limit}
	var resp honchoSearchResponse
	if err := httpJSON(ctx, "POST", url, h.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Content)
	}
	return out, nil
}

// RegisterTools registers honcho_recall and honcho_remember into reg.
func (h *Honcho) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "honcho_remember",
		Toolset:     "memory",
		Description: "Explicitly store a fact in Honcho for future recall.",
		Emoji:       "🪶",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "honcho_remember",
				Description: "Store a fact in Honcho (external memory provider).",
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
			if err := h.addMessage(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "honcho_recall",
		Toolset:     "memory",
		Description: "Recall relevant memories from Honcho by semantic query.",
		Emoji:       "🔎",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "honcho_recall",
				Description: "Search Honcho memories and return matching content.",
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
			results, err := h.search(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
