// tool/memory/memory.go
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// --- memory_save ---

const memorySaveSchema = `{
  "type": "object",
  "properties": {
    "content":  { "type": "string", "description": "The memory content to save" },
    "category": { "type": "string", "description": "Optional category (e.g., preference, fact, instruction)" },
    "tags":     { "type": "array", "items": {"type":"string"}, "description": "Optional tags" }
  },
  "required": ["content"]
}`

type memorySaveArgs struct {
	Content  string   `json:"content"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type memorySaveResult struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func newMemorySaveHandler(store storage.Storage) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args memorySaveArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.Content == "" {
			return tool.ToolError("content is required"), nil
		}
		now := time.Now().UTC()
		id := fmt.Sprintf("mem_%d", now.UnixNano())
		mem := &storage.Memory{
			ID:        id,
			Content:   args.Content,
			Category:  args.Category,
			Tags:      args.Tags,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := store.SaveMemory(ctx, mem); err != nil {
			return tool.ToolError("save failed: " + err.Error()), nil
		}
		return tool.ToolResult(memorySaveResult{ID: id, CreatedAt: now}), nil
	}
}

// --- memory_search ---

const memorySearchSchema = `{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "FTS search query (empty to list recent)" },
    "limit": { "type": "number", "description": "Max results (default 10)" }
  }
}`

type memorySearchArgs struct {
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type memorySearchResultItem struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Category  string    `json:"category,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type memorySearchResult struct {
	Query   string                   `json:"query"`
	Results []memorySearchResultItem `json:"results"`
}

func newMemorySearchHandler(store storage.Storage) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args memorySearchArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}
		mems, err := store.SearchMemories(ctx, args.Query, &storage.MemorySearchOptions{Limit: limit})
		if err != nil {
			return tool.ToolError("search failed: " + err.Error()), nil
		}
		items := make([]memorySearchResultItem, 0, len(mems))
		for _, m := range mems {
			items = append(items, memorySearchResultItem{
				ID:        m.ID,
				Content:   m.Content,
				Category:  m.Category,
				Tags:      m.Tags,
				CreatedAt: m.CreatedAt,
			})
		}
		return tool.ToolResult(memorySearchResult{Query: args.Query, Results: items}), nil
	}
}

// --- memory_delete ---

const memoryDeleteSchema = `{
  "type": "object",
  "properties": {
    "id": { "type": "string", "description": "Memory ID to delete" }
  },
  "required": ["id"]
}`

type memoryDeleteArgs struct {
	ID string `json:"id"`
}

func newMemoryDeleteHandler(store storage.Storage) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args memoryDeleteArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if args.ID == "" {
			return tool.ToolError("id is required"), nil
		}
		if err := store.DeleteMemory(ctx, args.ID); err != nil {
			return tool.ToolError("delete failed: " + err.Error()), nil
		}
		return tool.ToolResult(map[string]any{"ok": true, "id": args.ID}), nil
	}
}
