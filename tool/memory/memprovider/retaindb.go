package memprovider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

// RetainDB is a Provider backed by the RetainDB cloud API.
// It exposes retaindb_save and retaindb_search.
type RetainDB struct {
	cfg       config.RetainDBConfig
	sessionID string
}

func NewRetainDB(cfg config.RetainDBConfig) *RetainDB {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.retaindb.com"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Project == "" {
		cfg.Project = "hermind"
	}
	if cfg.UserID == "" {
		cfg.UserID = "default"
	}
	return &RetainDB{cfg: cfg}
}

func (r *RetainDB) Name() string { return "retaindb" }

func (r *RetainDB) Initialize(ctx context.Context, sessionID string) error {
	r.sessionID = sessionID
	return nil
}

func (r *RetainDB) Shutdown(ctx context.Context) error { return nil }

func (r *RetainDB) save(ctx context.Context, content string) error {
	url := r.cfg.BaseURL + "/v1/memory"
	body := map[string]any{
		"project": r.cfg.Project,
		"user_id": r.cfg.UserID,
		"content": content,
	}
	return httpJSON(ctx, "POST", url, r.cfg.APIKey, body, nil)
}

func (r *RetainDB) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return r.save(ctx, "user: "+userMsg+"\nassistant: "+assistantMsg)
}

type retaindbSearchResponse struct {
	Results []struct {
		Content string `json:"content"`
	} `json:"results"`
}

func (r *RetainDB) search(ctx context.Context, query string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	url := r.cfg.BaseURL + "/v1/memory/search"
	body := map[string]any{
		"project": r.cfg.Project,
		"user_id": r.cfg.UserID,
		"query":   query,
		"limit":   limit,
	}
	var resp retaindbSearchResponse
	if err := httpJSON(ctx, "POST", url, r.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Content)
	}
	return out, nil
}

func (r *RetainDB) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "retaindb_save",
		Toolset:     "memory",
		Description: "Save a fact to RetainDB.",
		Emoji:       "📥",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "retaindb_save",
				Description: "Store a fact in the RetainDB memory store.",
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
			if err := r.save(ctx, args.Content); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "retaindb_search",
		Toolset:     "memory",
		Description: "Search RetainDB memories.",
		Emoji:       "🔍",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "retaindb_search",
				Description: "Search RetainDB memories by text query.",
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
			results, err := r.search(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
