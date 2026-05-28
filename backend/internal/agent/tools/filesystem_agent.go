package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

const fsMaxReadBytes = 1 << 20 // 1 MiB per read

func NewFilesystemAgentSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "filesystem-agent",
		Toolset:        "filesystem",
		Description:    "Read, write, search, and organize files within the agent's sandboxed workspace.",
		Emoji:          "📂",
		MaxResultChars: 16 * 1024,
		CheckFn: func() bool {
			return tc.Cfg != nil && tc.Cfg.AgentFilesystemEnabled
		},
		Schema: core.ToolDefinition{
			Name:        "filesystem-agent",
			Description: "Sandboxed filesystem operations",
			Parameters:  filesystemAgentSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Action      string `json:"action"`
				Path        string `json:"path,omitempty"`
				Source      string `json:"source,omitempty"`
				Destination string `json:"destination,omitempty"`
				Content     string `json:"content,omitempty"`
				Pattern     string `json:"pattern,omitempty"`
				OldString   string `json:"old_string,omitempty"`
				NewString   string `json:"new_string,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if tc.Cfg == nil {
				return tool.Error("filesystem not configured"), nil
			}
			root := tc.Cfg.AgentFilesystemRoot

			// Approval-required actions
			destructive := map[string]bool{
				"write_file": true, "edit_file": true, "move_file": true, "copy_file": true, "create_dir": true,
			}
			if destructive[args.Action] && tc.Approval != nil {
				desc := fmt.Sprintf("Filesystem %s: %s", args.Action, args.Path+args.Source)
				if approved, reason := tc.Approval(ctx, "filesystem-agent:"+args.Action, args, desc); !approved {
					return tool.Error("rejected: " + reason), nil
				}
			}

			switch args.Action {
			case "list_dir":
				return fsListDir(tc, root, args.Path)
			case "read_file":
				return fsReadFile(tc, root, args.Path)
			case "get_info":
				return fsGetInfo(tc, root, args.Path)
			case "search_files":
				return fsSearchFiles(tc, root, args.Path, args.Pattern)
			case "write_file":
				return fsWriteFile(tc, root, args.Path, args.Content)
			case "edit_file":
				return fsEditFile(tc, root, args.Path, args.OldString, args.NewString)
			case "move_file":
				return fsMoveFile(tc, root, args.Source, args.Destination)
			case "copy_file":
				return fsCopyFile(tc, root, args.Source, args.Destination)
			case "create_dir":
				return fsCreateDir(tc, root, args.Path)
			default:
				return tool.Error("unknown action: " + args.Action), nil
			}
		},
	}
}

func filesystemAgentSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"action":      {"type": "string", "enum": ["list_dir","read_file","write_file","edit_file","move_file","copy_file","search_files","get_info","create_dir"]},
			"path":        {"type": "string"},
			"source":      {"type": "string"},
			"destination": {"type": "string"},
			"content":     {"type": "string"},
			"pattern":     {"type": "string"},
			"old_string":  {"type": "string"},
			"new_string":  {"type": "string"}
		},
		"required": ["action"]
	}`))
}

func fsListDir(tc *ToolContext, root, path string) (string, error) {
	abs, err := safeJoin(root, path)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	tc.Emit("Listing " + path)
	entries, err := os.ReadDir(abs)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		item := map[string]any{"name": e.Name(), "is_dir": e.IsDir()}
		if info != nil {
			item["size"] = info.Size()
		}
		out = append(out, item)
	}
	return tool.Result(map[string]any{"path": path, "entries": out}), nil
}

func fsReadFile(tc *ToolContext, root, path string) (string, error) {
	abs, err := safeJoin(root, path)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	tc.Emit("Reading " + path)
	info, err := os.Stat(abs)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	if info.IsDir() {
		return tool.Error("path is a directory: " + path), nil
	}
	f, err := os.Open(abs)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, fsMaxReadBytes))
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	truncated := info.Size() > fsMaxReadBytes
	return tool.Result(map[string]any{
		"path":      path,
		"content":   string(body),
		"size":      info.Size(),
		"truncated": truncated,
	}), nil
}

func fsGetInfo(tc *ToolContext, root, path string) (string, error) {
	abs, err := safeJoin(root, path)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	info, err := os.Stat(abs)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	return tool.Result(map[string]any{
		"path":     path,
		"size":     info.Size(),
		"is_dir":   info.IsDir(),
		"modified": info.ModTime(),
	}), nil
}

func fsSearchFiles(tc *ToolContext, root, path, pattern string) (string, error) {
	if pattern == "" {
		return tool.Error("pattern is required"), nil
	}
	abs, err := safeJoin(root, path)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	tc.Emit(fmt.Sprintf("Searching for %q under %s", pattern, path))
	var matches []string
	filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			ok, _ := filepath.Match(pattern, d.Name())
			if ok {
				rel, _ := filepath.Rel(abs, p)
				matches = append(matches, filepath.Join(path, rel))
			}
		}
		if len(matches) >= 200 {
			return filepath.SkipAll
		}
		return nil
	})
	return tool.Result(map[string]any{"pattern": pattern, "matches": matches, "limit_hit": len(matches) == 200}), nil
}

func fsWriteFile(tc *ToolContext, root, path, content string) (string, error) {
	abs, err := safeJoin(root, path)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return tool.Error(err.Error()), nil
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return tool.Error(err.Error()), nil
	}
	tc.Emit("Wrote " + path)
	return tool.Result(map[string]any{"path": path, "bytes": len(content)}), nil
}

func fsEditFile(tc *ToolContext, root, path, oldStr, newStr string) (string, error) {
	if oldStr == "" {
		return tool.Error("old_string is required"), nil
	}
	abs, err := safeJoin(root, path)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	count := 0
	for i := 0; i < len(body); {
		idx := 0
		for j := range oldStr {
			if i+j >= len(body) || body[i+j] != oldStr[j] {
				break
			}
			idx = j + 1
		}
		if idx == len(oldStr) {
			count++
			i += idx
		} else {
			i++
		}
	}
	switch count {
	case 0:
		return tool.Error("old_string not found in file"), nil
	case 1:
		newBody := replaceOnce(string(body), oldStr, newStr)
		if err := os.WriteFile(abs, []byte(newBody), 0o644); err != nil {
			return tool.Error(err.Error()), nil
		}
		tc.Emit("Edited " + path)
		return tool.Result(map[string]any{"path": path, "replacements": 1}), nil
	default:
		return tool.Error(fmt.Sprintf("old_string is ambiguous (%d occurrences); provide more context", count)), nil
	}
}

func replaceOnce(s, old, new string) string {
	for i := 0; i < len(s); {
		match := true
		for j := range old {
			if i+j >= len(s) || s[i+j] != old[j] {
				match = false
				break
			}
		}
		if match {
			return s[:i] + new + s[i+len(old):]
		}
		i++
	}
	return s
}

func fsMoveFile(tc *ToolContext, root, src, dst string) (string, error) {
	absSrc, err := safeJoin(root, src)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	absDst, err := safeJoin(root, dst)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	if err := os.MkdirAll(filepath.Dir(absDst), 0o755); err != nil {
		return tool.Error(err.Error()), nil
	}
	if err := os.Rename(absSrc, absDst); err != nil {
		return tool.Error(err.Error()), nil
	}
	tc.Emit("Moved " + src + " → " + dst)
	return tool.Result(map[string]any{"from": src, "to": dst}), nil
}

func fsCopyFile(tc *ToolContext, root, src, dst string) (string, error) {
	absSrc, err := safeJoin(root, src)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	absDst, err := safeJoin(root, dst)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	if err := os.MkdirAll(filepath.Dir(absDst), 0o755); err != nil {
		return tool.Error(err.Error()), nil
	}
	data, err := os.ReadFile(absSrc)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	if err := os.WriteFile(absDst, data, 0o644); err != nil {
		return tool.Error(err.Error()), nil
	}
	tc.Emit("Copied " + src + " → " + dst)
	return tool.Result(map[string]any{"from": src, "to": dst, "bytes": len(data)}), nil
}

func fsCreateDir(tc *ToolContext, root, path string) (string, error) {
	abs, err := safeJoin(root, path)
	if err != nil {
		return tool.Error(err.Error()), nil
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return tool.Error(err.Error()), nil
	}
	tc.Emit("Created dir " + path)
	return tool.Result(map[string]any{"path": path}), nil
}
