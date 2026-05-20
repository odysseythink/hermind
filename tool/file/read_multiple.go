package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const readMultipleFilesSchema = `{
  "type": "object",
  "properties": {
    "paths": {
      "type": "array",
      "items": { "type": "string" },
      "description": "List of file paths to read."
    }
  },
  "required": ["paths"]
}`

type readMultipleFilesArgs struct {
	Paths []string `json:"paths"`
}

type readMultipleResult struct {
	Files []fileContent `json:"files"`
}

type fileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Error   string `json:"error,omitempty"`
}

const maxReadMultipleFiles = 50
const maxReadMultipleBytes = 1 << 20 // 1 MiB per file

func readMultipleFilesHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args readMultipleFilesArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if len(args.Paths) == 0 {
		return tool.ToolError("paths array is required"), nil
	}
	if len(args.Paths) > maxReadMultipleFiles {
		return tool.ToolError(fmt.Sprintf("too many files: max %d", maxReadMultipleFiles)), nil
	}

	allowed := getAllowedDirs(cfg)
	out := readMultipleResult{Files: make([]fileContent, 0, len(args.Paths))}

	for _, p := range args.Paths {
		if err := validatePath(p, allowed); err != nil {
			out.Files = append(out.Files, fileContent{Path: p, Error: err.Error()})
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			out.Files = append(out.Files, fileContent{Path: p, Error: err.Error()})
			continue
		}
		if info.IsDir() {
			out.Files = append(out.Files, fileContent{Path: p, Error: "is a directory"})
			continue
		}
		if info.Size() > maxReadMultipleBytes {
			out.Files = append(out.Files, fileContent{Path: p, Error: fmt.Sprintf("file too large: %d bytes", info.Size())})
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			out.Files = append(out.Files, fileContent{Path: p, Error: err.Error()})
			continue
		}
		content := string(data)
		if len(content) > 100000 {
			content = content[:100000] + "\n[Content truncated]"
		}
		out.Files = append(out.Files, fileContent{
			Path:    p,
			Content: content,
			Size:    len(data),
		})
	}

	return tool.ToolResult(out), nil
}
