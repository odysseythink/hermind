package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nousresearch/hermes-agent/tool"
)

const writeFileSchema = `{
  "type": "object",
  "properties": {
    "path":        { "type": "string", "description": "File path to write." },
    "content":     { "type": "string", "description": "Content to write." },
    "create_dirs": { "type": "boolean", "description": "Create parent directories if missing. Default false." }
  },
  "required": ["path", "content"]
}`

type writeFileArgs struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	CreateDirs bool   `json:"create_dirs"`
}

type writeFileResult struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

func writeFileHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args writeFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	if args.CreateDirs {
		if dir := filepath.Dir(args.Path); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return tool.ToolError(fmt.Sprintf("mkdir %s: %s", dir, err.Error())), nil
			}
		}
	}

	if err := os.WriteFile(args.Path, []byte(args.Content), 0o644); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(writeFileResult{
		Path:         args.Path,
		BytesWritten: len(args.Content),
	}), nil
}
