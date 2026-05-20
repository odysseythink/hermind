package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const moveFileSchema = `{
  "type": "object",
  "properties": {
    "source":      { "type": "string", "description": "Source file path." },
    "destination": { "type": "string", "description": "Destination file path." }
  },
  "required": ["source", "destination"]
}`

type moveFileArgs struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type moveFileResult struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func moveFileHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args moveFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	allowed := getAllowedDirs(cfg)
	if err := validatePath(args.Source, allowed); err != nil {
		return tool.ToolError("source: " + err.Error()), nil
	}
	if err := validatePath(args.Destination, allowed); err != nil {
		return tool.ToolError("destination: " + err.Error()), nil
	}

	if err := os.Rename(args.Source, args.Destination); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(moveFileResult{
		Source:      args.Source,
		Destination: args.Destination,
	}), nil
}
