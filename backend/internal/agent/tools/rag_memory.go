package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func NewRAGMemorySkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "rag-memory",
		Toolset:        "memory",
		Description:    "Search local documents or store information to long-term memory. Action 'search' finds relevant passages; 'store' saves content for later retrieval.",
		Emoji:          "🧠",
		MaxResultChars: 8 * 1024,
		CheckFn:        func() bool { return tc.VectorSearchSvc != nil },
		Schema: core.ToolDefinition{
			Name:        "rag-memory",
			Description: "Search or store workspace memory",
			Parameters:  ragMemorySchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Action  string `json:"action"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return "", err
			}
			switch args.Action {
			case "search":
				return ragMemorySearch(ctx, tc, args.Content)
			case "store":
				return ragMemoryStore(ctx, tc, args.Content)
			default:
				return tool.Error(fmt.Sprintf("unknown action %q", args.Action)), nil
			}
		},
	}
}

func ragMemorySearch(ctx context.Context, tc *ToolContext, content string) (string, error) {
	tc.Emit("Searching memory: " + truncate(content, 60))
	results, err := tc.VectorSearchSvc.Search(ctx, tc.Workspace, dto.VectorSearchRequest{
		Query: content,
		TopN:  intPtr(4),
	})
	if err != nil {
		return tool.Error("vector search: " + err.Error()), nil
	}
	if len(results) == 0 {
		return `{"results":[]}`, nil
	}
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		source := ""
		if r.Metadata != nil {
			if s, ok := r.Metadata["source"].(string); ok {
				source = s
			} else if s, ok := r.Metadata["sourceName"].(string); ok {
				source = s
			}
		}
		out = append(out, map[string]any{
			"text":   r.Text,
			"score":  r.Score,
			"source": source,
		})
	}
	b, _ := json.Marshal(map[string]any{"results": out})
	return string(b), nil
}

func ragMemoryStore(ctx context.Context, tc *ToolContext, content string) (string, error) {
	tc.Emit("Memory store request acknowledged (deferred)")
	return tool.Result(map[string]any{"status": "deferred", "note": "store action is not yet implemented"}), nil
}

func ragMemorySchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"action":  {"type": "string", "enum": ["search", "store"]},
			"content": {"type": "string"}
		},
		"required": ["action", "content"]
	}`))
}

func intPtr(n int) *int { return &n }
