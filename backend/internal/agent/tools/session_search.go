package tools

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

// ChatSearcher is the interface for searching workspace chat history.
type ChatSearcher interface {
	SearchWorkspaceChatsFTS5(ctx context.Context, workspaceID int, query string, limit int) ([]models.WorkspaceChat, error)
}

func NewSessionSearchSkill(tc *ToolContext, searcher ChatSearcher) *tool.Entry {
	return &tool.Entry{
		Name:           "session-search",
		Toolset:        "memory",
		Description:    "Search past conversations in this workspace for relevant context.",
		MaxResultChars: 8 * 1024,
		Schema: core.ToolDefinition{
			Name:        "session-search",
			Description: "Search past conversations in this workspace for relevant context",
			Parameters:  sessionSearchSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if args.Query == "" {
				return tool.Error("query is required"), nil
			}
			if args.Limit <= 0 {
				args.Limit = 5
			}
			if args.Limit > 20 {
				args.Limit = 20
			}

			tc.Emit("Searching chat history for: " + args.Query)

			results, err := searcher.SearchWorkspaceChatsFTS5(ctx, tc.Workspace.ID, args.Query, args.Limit)
			if err != nil {
				return tool.Error("search failed: " + err.Error()), nil
			}

			items := make([]map[string]any, 0, len(results))
			for _, r := range results {
				items = append(items, map[string]any{
					"id":         r.ID,
					"prompt":     r.Prompt,
					"response":   r.Response,
					"created_at": r.CreatedAt.Format("2006-01-02T15:04:05Z"),
				})
			}

			return tool.Result(map[string]any{
				"query":   args.Query,
				"results": items,
			}), nil
		},
	}
}

func sessionSearchSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query for past conversations"},
			"limit": {"type": "integer", "description": "Maximum number of results (default 5, max 20)", "default": 5}
		},
		"required": ["query"]
	}`))
}
