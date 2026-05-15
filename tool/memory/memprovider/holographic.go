package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// Holographic is a Provider that layers a distinct tool surface
// (holographic_remember / holographic_recall) on top of the shared
// storage.Storage. It does NOT implement the HRR / entity-resolution
// math from the Python reference — it is a minimal local-SQLite
// adapter so users can exercise the provider interface without a
// cloud account.
type Holographic struct {
	store     storage.Storage
	sessionID string
}

func NewHolographic(store storage.Storage) *Holographic {
	return &Holographic{store: store}
}

func (h *Holographic) Name() string { return "holographic" }

func (h *Holographic) Initialize(ctx context.Context, sessionID string) error {
	if h.store == nil {
		return fmt.Errorf("holographic: storage is required")
	}
	h.sessionID = sessionID
	return nil
}

func (h *Holographic) Shutdown(ctx context.Context) error { return nil }

func (h *Holographic) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	return h.save(ctx, "user: "+userMsg+"\nassistant: "+assistantMsg, "conversation")
}

func (h *Holographic) save(ctx context.Context, content, category string) error {
	now := time.Now().UTC()
	id := fmt.Sprintf("holo_%d", now.UnixNano())
	return h.store.SaveMemory(ctx, &storage.Memory{
		ID:        id,
		Content:   content,
		Category:  category,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (h *Holographic) recall(ctx context.Context, query string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}
	mems, err := h.store.SearchMemories(ctx, query, &storage.MemorySearchOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(mems))
	for _, m := range mems {
		out = append(out, m.Content)
	}
	return out, nil
}

func (h *Holographic) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "holographic_remember",
		Toolset:     "memory",
		Description: "Store a fact in the local Holographic memory store.",
		Emoji:       "🌀",
		Schema: core.ToolDefinition{
			Name:        "holographic_remember",
			Description: "Store a fact in the local Holographic (SQLite) memory store.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
  "type":"object",
  "properties":{
    "content":{"type":"string"},
    "category":{"type":"string"}
  },
  "required":["content"]
}`)),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Content  string `json:"content"`
				Category string `json:"category,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := h.save(ctx, args.Content, args.Category); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "holographic_recall",
		Toolset:     "memory",
		Description: "Recall items from the local Holographic memory store.",
		Emoji:       "🪩",
		Schema: core.ToolDefinition{
			Name:        "holographic_recall",
			Description: "Search local Holographic memories by full-text query.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
  "type":"object",
  "properties":{
    "query":{"type":"string"},
    "limit":{"type":"number"}
  },
  "required":["query"]
}`)),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			results, err := h.recall(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
