package obsidian

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const readNoteSchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Relative path to the note within the vault" }
  },
  "required": ["path"]
}`

type readNoteArgs struct {
	Path string `json:"path"`
}

type readNoteResult struct {
	Path        string         `json:"path"`
	FrontMatter map[string]any `json:"front_matter"`
	Body        string         `json:"body"`
	Links       []string       `json:"links"`
}

func readNoteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args readNoteArgs
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

	data, err := os.ReadFile(resolved)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	fm, body, err := parseFrontMatter(string(data))
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(readNoteResult{
		Path:        args.Path,
		FrontMatter: fm,
		Body:        body,
		Links:       extractWikilinks(body),
	}), nil
}
