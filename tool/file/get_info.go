package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const getFileInfoSchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "File or directory path." }
  },
  "required": ["path"]
}`

type getFileInfoArgs struct {
	Path string `json:"path"`
}

type getFileInfoResult struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	IsDir       bool   `json:"is_dir"`
	Mode        string `json:"mode"`
	ModTime     string `json:"mod_time"`
	Permissions string `json:"permissions"`
}

func getFileInfoHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args getFileInfoArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if err := validatePath(args.Path, getAllowedDirs(cfg)); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	info, err := os.Stat(args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(getFileInfoResult{
		Path:        args.Path,
		Size:        info.Size(),
		IsDir:       info.IsDir(),
		Mode:        info.Mode().String(),
		ModTime:     info.ModTime().Format("2006-01-02 15:04:05"),
		Permissions: info.Mode().Perm().String(),
	}), nil
}
