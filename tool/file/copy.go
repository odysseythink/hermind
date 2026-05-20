package file

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const copyFileSchema = `{
  "type": "object",
  "properties": {
    "source":      { "type": "string", "description": "Source file path." },
    "destination": { "type": "string", "description": "Destination file path." }
  },
  "required": ["source", "destination"]
}`

type copyFileArgs struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type copyFileResult struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	BytesCopied int64  `json:"bytes_copied"`
}

func copyFileHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args copyFileArgs
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

	src, err := os.Open(args.Source)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}
	defer src.Close()

	dst, err := os.Create(args.Destination)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(copyFileResult{
		Source:      args.Source,
		Destination: args.Destination,
		BytesCopied: n,
	}), nil
}
