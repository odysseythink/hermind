package file

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/odysseythink/hermind/tool"
)

const editFileSchema = `{
  "type": "object",
  "properties": {
    "path":        { "type": "string", "description": "File path to edit." },
    "old_string":  { "type": "string", "description": "Text to find and replace." },
    "new_string":  { "type": "string", "description": "Replacement text." }
  },
  "required": ["path", "old_string", "new_string"]
}`

type editFileArgs struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type editFileResult struct {
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
}

func editFileHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args editFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if err := validatePath(args.Path, getAllowedDirs(cfg)); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	content := string(data)
	if args.OldString == "" {
		return tool.ToolError("old_string cannot be empty"), nil
	}
	if !strings.Contains(content, args.OldString) {
		return tool.ToolError("old_string not found in file"), nil
	}

	newContent := strings.ReplaceAll(content, args.OldString, args.NewString)
	replacements := strings.Count(content, args.OldString)

	if err := os.WriteFile(args.Path, []byte(newContent), 0o644); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(editFileResult{
		Path:         args.Path,
		Replacements: replacements,
	}), nil
}
