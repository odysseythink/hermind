package obsidian

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const updateFrontMatterSchema = `{
  "type": "object",
  "properties": {
    "path":    { "type": "string", "description": "Relative path to the note" },
    "updates": { "type": "object", "description": "Key-value map to merge into front-matter" }
  },
  "required": ["path", "updates"]
}`

type updateFrontMatterArgs struct {
	Path    string         `json:"path"`
	Updates map[string]any `json:"updates"`
}

type updateFrontMatterResult struct {
	Path        string         `json:"path"`
	FrontMatter map[string]any `json:"front_matter"`
}

func updateFrontMatterHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args updateFrontMatterArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" || len(args.Updates) == 0 {
		return tool.ToolError("path and updates are required"), nil
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

	for k, v := range args.Updates {
		fm[k] = v
	}

	out, err := serializeNote(fm, body)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	if err := os.WriteFile(resolved, []byte(out), 0o644); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(updateFrontMatterResult{
		Path:        args.Path,
		FrontMatter: fm,
	}), nil
}
