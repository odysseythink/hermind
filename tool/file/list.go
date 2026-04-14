package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const listDirectorySchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Directory path." }
  },
  "required": ["path"]
}`

type listDirectoryArgs struct {
	Path string `json:"path"`
}

type dirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

type listDirectoryResult struct {
	Path    string     `json:"path"`
	Entries []dirEntry `json:"entries"`
}

func listDirectoryHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args listDirectoryArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	entries, err := os.ReadDir(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	out := listDirectoryResult{Path: args.Path, Entries: make([]dirEntry, 0, len(entries))}
	for _, e := range entries {
		de := dirEntry{Name: e.Name(), IsDir: e.IsDir()}
		if info, err := e.Info(); err == nil && !e.IsDir() {
			de.Size = info.Size()
		}
		out.Entries = append(out.Entries, de)
	}
	return tool.ToolResult(out), nil
}
