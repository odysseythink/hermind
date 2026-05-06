package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/odysseythink/hermind/tool"
)

const writeNoteSchema = `{
  "type": "object",
  "properties": {
    "path":        { "type": "string", "description": "Relative path within the vault" },
    "content":     { "type": "string", "description": "Markdown body" },
    "frontmatter": { "type": "object", "description": "Optional front-matter key-value map" }
  },
  "required": ["path", "content"]
}`

type writeNoteArgs struct {
	Path        string         `json:"path"`
	Content     string         `json:"content"`
	FrontMatter map[string]any `json:"frontmatter,omitempty"`
}

type writeNoteResult struct {
	Path string `json:"path"`
}

func writeNoteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args writeNoteArgs
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

	// Backup existing file before overwrite
	if _, err := os.Stat(resolved); err == nil {
		backupDir := filepath.Join(vaultPath, ".hermind", "obsidian-backups")
		_ = os.MkdirAll(backupDir, 0o755)
		backupPath := filepath.Join(backupDir, filepath.Base(args.Path)+".backup")
		b, _ := os.ReadFile(resolved)
		_ = os.WriteFile(backupPath, b, 0o644)
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return tool.ToolError(fmt.Sprintf("mkdir: %s", err)), nil
	}

	out, err := serializeNote(args.FrontMatter, args.Content)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	if err := os.WriteFile(resolved, []byte(out), 0o644); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(writeNoteResult{Path: args.Path}), nil
}
