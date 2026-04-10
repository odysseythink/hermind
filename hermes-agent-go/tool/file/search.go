package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/nousresearch/hermes-agent/tool"
)

const searchFilesSchema = `{
  "type": "object",
  "properties": {
    "root":    { "type": "string", "description": "Directory to search in (recursive)." },
    "pattern": { "type": "string", "description": "Glob pattern, e.g. '*.go' or 'main.*'. Matched against filename only." }
  },
  "required": ["root", "pattern"]
}`

type searchFilesArgs struct {
	Root    string `json:"root"`
	Pattern string `json:"pattern"`
}

type searchMatch struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type searchFilesResult struct {
	Root    string        `json:"root"`
	Pattern string        `json:"pattern"`
	Matches []searchMatch `json:"matches"`
}

const maxSearchMatches = 500

func searchFilesHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	var args searchFilesArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Root == "" || args.Pattern == "" {
		return tool.ToolError("root and pattern are required"), nil
	}

	var matches []searchMatch
	err := filepath.WalkDir(args.Root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries, keep walking
		}
		if d.IsDir() {
			return nil
		}
		ok, matchErr := filepath.Match(args.Pattern, d.Name())
		if matchErr != nil {
			return matchErr
		}
		if !ok {
			return nil
		}
		info, _ := d.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		matches = append(matches, searchMatch{Path: path, Size: size})
		if len(matches) >= maxSearchMatches {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	if matches == nil {
		matches = []searchMatch{}
	}
	return tool.ToolResult(searchFilesResult{
		Root:    args.Root,
		Pattern: args.Pattern,
		Matches: matches,
	}), nil
}
