package obsidian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/tool"
)

const listLinksSchema = `{
  "type": "object",
  "properties": {
    "path":      { "type": "string", "description": "Relative path to the note" },
    "direction": { "type": "string", "enum": ["outgoing", "incoming", "both"], "description": "Which links to return" }
  },
  "required": ["path", "direction"]
}`

type listLinksArgs struct {
	Path      string `json:"path"`
	Direction string `json:"direction"`
}

type listLinksResult struct {
	Outgoing []string `json:"outgoing,omitempty"`
	Incoming []string `json:"incoming,omitempty"`
}

func listLinksHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args listLinksArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" || args.Direction == "" {
		return tool.ToolError("path and direction are required"), nil
	}

	resolved, err := resolveVaultPath(vaultPath, args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	result := listLinksResult{}

	if args.Direction == "outgoing" || args.Direction == "both" {
		result.Outgoing = extractWikilinks(string(data))
	}

	if args.Direction == "incoming" || args.Direction == "both" {
		base := strings.TrimSuffix(filepath.Base(args.Path), ".md")
		_ = filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") || path == resolved {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			links := extractWikilinks(string(content))
			for _, link := range links {
				if strings.EqualFold(link, base) {
					rel, _ := filepath.Rel(vaultPath, path)
					result.Incoming = append(result.Incoming, rel)
					break
				}
			}
			return nil
		})
	}

	return tool.ToolResult(result), nil
}
