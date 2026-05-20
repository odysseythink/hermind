package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/odysseythink/hermind/tool"
)

const readFileSchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Absolute or relative file path." },
    "head": { "type": "integer", "description": "If provided, returns only the first N lines." },
    "tail": { "type": "integer", "description": "If provided, returns only the last N lines." }
  },
  "required": ["path"]
}`

type readFileArgs struct {
	Path string `json:"path"`
	Head *int   `json:"head,omitempty"`
	Tail *int   `json:"tail,omitempty"`
}

type readFileResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
}

// readFileHandler reads the file at the given path and returns its contents.
// Files larger than 1 MiB are refused to protect the context window.
const maxReadFileBytes = 1 << 20

func readFileHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args readFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}
	if err := validatePath(args.Path, getAllowedDirs(cfg)); err != nil {
		return tool.ToolError(err.Error()), nil
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

	content := string(data)
	if args.Head != nil && args.Tail != nil {
		return tool.ToolError("cannot specify both head and tail"), nil
	}
	if args.Head != nil && *args.Head > 0 {
		lines := strings.Split(content, "\n")
		if *args.Head < len(lines) {
			lines = lines[:*args.Head]
		}
		content = strings.Join(lines, "\n")
	}
	if args.Tail != nil && *args.Tail > 0 {
		lines := strings.Split(content, "\n")
		if *args.Tail < len(lines) {
			lines = lines[len(lines)-*args.Tail:]
		}
		content = strings.Join(lines, "\n")
	}

	return tool.ToolResult(readFileResult{
		Path:    args.Path,
		Content: content,
		Size:    len(data),
	}), nil
}
