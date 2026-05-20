package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

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
	allowed := getAllowedDirs(cfg)
	if err := validatePath(args.Path, allowed); err != nil {
		parent := filepath.Dir(args.Path)
		if parentErr := validatePath(parent, allowed); parentErr != nil {
			return tool.ToolError(err.Error()), nil
		}
	}

	if err := os.MkdirAll(args.Path, 0o755); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(createDirectoryResult{Path: args.Path}), nil
}
