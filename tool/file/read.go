package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const readFileSchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Absolute or relative file path." }
  },
  "required": ["path"]
}`

type readFileArgs struct {
	Path string `json:"path"`
}

type readFileResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
}

// readFileHandler reads the file at the given path and returns its contents.
// Files larger than 1 MiB are refused to protect the context window.
const maxReadFileBytes = 1 << 20

func readFileHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args readFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	info, err := os.Stat(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}
	if info.IsDir() {
		return tool.ToolError(fmt.Sprintf("%s is a directory, use list_directory", args.Path)), nil
	}
	if info.Size() > maxReadFileBytes {
		return tool.ToolError(fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxReadFileBytes)), nil
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(readFileResult{
		Path:    args.Path,
		Content: string(data),
		Size:    len(data),
	}), nil
}
