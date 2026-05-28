package tools

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func mcpToolToEntry(p mcp.ToolPlugin, emit StatusEmitter) *tool.Entry {
	var params *core.Schema
	if len(p.InputSchema) > 0 {
		params = buildSchema(p.InputSchema)
	}
	return &tool.Entry{
		Name:           p.QualifiedName,
		Toolset:        "mcp",
		Description:    p.Description,
		MaxResultChars: 8 * 1024,
		Schema: core.ToolDefinition{
			Name:        p.QualifiedName,
			Description: p.Description,
			Parameters:  params,
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			emit("Calling MCP tool: " + p.QualifiedName)
			var args map[string]any
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return tool.Error("invalid args: " + err.Error()), nil
				}
			}
			result, err := p.Call(ctx, args)
			if err != nil {
				return tool.Error(err.Error()), nil
			}
			b, mErr := json.Marshal(result)
			if mErr != nil {
				return tool.Error("marshal result: " + mErr.Error()), nil
			}
			return string(b), nil
		},
	}
}
