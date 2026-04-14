package memprovider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

// OpenViking is a Provider backed by an OpenViking server.
// It maintains a single active session ID per provider instance and
// exposes openviking_find and openviking_append.
type OpenViking struct {
	cfg       config.OpenVikingConfig
	sessionID string
}

func NewOpenViking(cfg config.OpenVikingConfig) *OpenViking {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:8787"
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	return &OpenViking{cfg: cfg}
}

func (o *OpenViking) Name() string { return "openviking" }

func (o *OpenViking) Initialize(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	o.sessionID = sessionID
	return nil
}

func (o *OpenViking) Shutdown(ctx context.Context) error { return nil }

func (o *OpenViking) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return o.append(ctx, "user: "+userMsg+"\nassistant: "+assistantMsg)
}

func (o *OpenViking) append(ctx context.Context, content string) error {
	url := o.cfg.Endpoint + "/api/v1/sessions/" + o.sessionID + "/messages"
	body := map[string]any{"content": content}
	return httpJSON(ctx, "POST", url, o.cfg.APIKey, body, nil)
}

type openVikingSearchResponse struct {
	Results []struct {
		Content string `json:"content"`
	} `json:"results"`
}

func (o *OpenViking) find(ctx context.Context, query string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := o.cfg.Endpoint + "/api/v1/search/find"
	body := map[string]any{"query": query, "limit": limit}
	var resp openVikingSearchResponse
	if err := httpJSON(ctx, "POST", url, o.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Content)
	}
	return out, nil
}

func (o *OpenViking) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "openviking_append",
		Toolset:     "memory",
		Description: "Append a message to the active OpenViking session.",
		Emoji:       "📎",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "openviking_append",
				Description: "Record a note/message in the OpenViking session.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"content":{"type":"string"}},
  "required":["content"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ Content string `json:"content"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := o.append(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "openviking_find",
		Toolset:     "memory",
		Description: "Search OpenViking for relevant items.",
		Emoji:       "🛶",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "openviking_find",
				Description: "Full-text search against OpenViking.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "query":{"type":"string"},
    "limit":{"type":"number"}
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
			results, err := o.find(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
