package memprovider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

// Hindsight is a Provider backed by the Hindsight cloud API.
// Exposes three tools: hindsight_retain, hindsight_recall,
// hindsight_reflect. Local embedded mode is out of scope for Plan 6c.1.
type Hindsight struct {
	cfg       config.HindsightConfig
	sessionID string
}

func NewHindsight(cfg config.HindsightConfig) *Hindsight {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.hindsight.vectorize.io"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.BankID == "" {
		cfg.BankID = "hermes"
	}
	if cfg.Budget == "" {
		cfg.Budget = "mid"
	}
	return &Hindsight{cfg: cfg}
}

func (h *Hindsight) Name() string { return "hindsight" }

func (h *Hindsight) Initialize(ctx context.Context, sessionID string) error {
	h.sessionID = sessionID
	return nil
}

func (h *Hindsight) Shutdown(ctx context.Context) error { return nil }

func (h *Hindsight) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return h.retain(ctx, "user: "+userMsg+"\nassistant: "+assistantMsg, "conversation")
}

func (h *Hindsight) retain(ctx context.Context, content, ctxLabel string) error {
	url := h.cfg.BaseURL + "/banks/" + h.cfg.BankID + "/retain"
	body := map[string]any{"content": content, "context": ctxLabel}
	return httpJSON(ctx, "POST", url, h.cfg.APIKey, body, nil)
}

type hindsightSearchResponse struct {
	Results []struct {
		Content string  `json:"content"`
		Score   float64 `json:"score,omitempty"`
	} `json:"results"`
}

func (h *Hindsight) recall(ctx context.Context, query string) ([]string, error) {
	url := h.cfg.BaseURL + "/banks/" + h.cfg.BankID + "/recall"
	body := map[string]any{"query": query, "budget": h.cfg.Budget}
	var resp hindsightSearchResponse
	if err := httpJSON(ctx, "POST", url, h.cfg.APIKey, body, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Content)
	}
	return out, nil
}

type hindsightReflectResponse struct {
	Answer string `json:"answer"`
}

func (h *Hindsight) reflect(ctx context.Context, query string) (string, error) {
	url := h.cfg.BaseURL + "/banks/" + h.cfg.BankID + "/reflect"
	body := map[string]any{"query": query, "budget": h.cfg.Budget}
	var resp hindsightReflectResponse
	if err := httpJSON(ctx, "POST", url, h.cfg.APIKey, body, &resp); err != nil {
		return "", err
	}
	return resp.Answer, nil
}

// RegisterTools registers hindsight_retain, hindsight_recall, and
// hindsight_reflect into reg.
func (h *Hindsight) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "hindsight_retain",
		Toolset:     "memory",
		Description: "Store information in Hindsight long-term memory.",
		Emoji:       "🗂",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "hindsight_retain",
				Description: "Store info to long-term memory with optional context label.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "content":{"type":"string","description":"Text to remember"},
    "context":{"type":"string","description":"Short label"}
  },
  "required":["content"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Content string `json:"content"`
				Context string `json:"context,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := h.retain(ctx, args.Content, args.Context); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "hindsight_recall",
		Toolset:     "memory",
		Description: "Search Hindsight long-term memory.",
		Emoji:       "🧭",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "hindsight_recall",
				Description: "Search long-term memory for relevant stored facts.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"query":{"type":"string"}},
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			results, err := h.recall(ctx, args.Query)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "hindsight_reflect",
		Toolset:     "memory",
		Description: "Synthesize a reasoned answer from Hindsight memories.",
		Emoji:       "💭",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "hindsight_reflect",
				Description: "Reason across stored memories to produce a coherent answer.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"query":{"type":"string"}},
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			answer, err := h.reflect(ctx, args.Query)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"answer": answer}), nil
		},
	})
}
