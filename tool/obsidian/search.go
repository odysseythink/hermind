package obsidian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/tool"
)

const searchVaultSchema = `{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "Text to search for in note contents" }
  },
  "required": ["query"]
}`

type searchVaultArgs struct {
	Query string `json:"query"`
}

type searchHit struct {
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
}

type searchVaultResult struct {
	Hits []searchHit `json:"hits"`
}

func searchVaultHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args searchVaultArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Query == "" {
		return tool.ToolError("query is required"), nil
	}

	var hits []searchHit
	lowerQuery := strings.ToLower(args.Query)

	err := filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if strings.Contains(strings.ToLower(content), lowerQuery) {
			rel, _ := filepath.Rel(vaultPath, path)
			snippet := extractSnippet(content, args.Query, 120)
			hits = append(hits, searchHit{Path: rel, Snippet: snippet})
		}
		return nil
	})
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(searchVaultResult{Hits: hits}), nil
}

func extractSnippet(content, query string, maxLen int) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	idx := strings.Index(lowerContent, lowerQuery)
	if idx < 0 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}
	start := idx - maxLen/2
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + maxLen/2
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}
