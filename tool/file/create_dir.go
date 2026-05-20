package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const createDirectorySchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Directory path to create." }
  },
  "required": ["path"]
}`

type createDirectoryArgs struct {
	Path string `json:"path"`
}

type createDirectoryResult struct {
	Path string `json:"path"`
}

func createDirectoryHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args createDirectoryArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if err := validatePath(args.Path, getAllowedDirs(cfg)); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	if err := os.MkdirAll(args.Path, 0o755); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(createDirectoryResult{Path: args.Path}), nil
}
