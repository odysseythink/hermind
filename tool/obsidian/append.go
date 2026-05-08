package obsidian

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const appendToNoteSchema = `{
  "type": "object",
  "properties": {
    "path":    { "type": "string", "description": "Relative path to the note" },
    "content": { "type": "string", "description": "Content to append" }
  },
  "required": ["path", "content"]
}`

type appendToNoteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type appendToNoteResult struct {
	Path string `json:"path"`
}

func appendToNoteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args appendToNoteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	resolved, err := resolveVaultPath(vaultPath, args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	f, err := os.OpenFile(resolved, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}
	defer f.Close()

	if _, err := f.WriteString(args.Content); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(appendToNoteResult{Path: args.Path}), nil
}
