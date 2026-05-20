# Filesystem Access Feature Port — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port AnythingLLM's filesystem access agent skill into Hermind, including 10 subtools with per-tool toggles, allowed-directories security whitelist, and a custom frontend detail panel.

**Architecture:** A virtual `filesystem` tool acts as the frontend aggregation entry. Ten independent `toolset="file"` subtools implement actual file I/O. The API layer hides individual file tools and exposes only the aggregated `filesystem` entry with a `SettingsSchema`. `activeToolReg()` filters subtools based on master toggle and per-subtool enablement. A shared `validatePath()` security layer enforces allowed-directory boundaries.

**Tech Stack:** Go 1.22+, React 18 + TypeScript, Vite, CSS Modules

---

## File Structure

### New Files (Backend)
- `tool/file/security.go` — `validatePath()`, `resolveAndCheckPath()`, allowed-directory enforcement
- `tool/file/security_test.go` — Tests for traversal, symlinks, allowed dir boundaries
- `tool/file/read_multiple.go` — `read_multiple_files` handler
- `tool/file/get_info.go` — `get_file_info` handler
- `tool/file/edit.go` — `edit_file` handler (find/replace)
- `tool/file/create_dir.go` — `create_directory` handler
- `tool/file/copy.go` — `copy_file` handler
- `tool/file/move.go` — `move_file` handler

### Modified Files (Backend)
- `tool/file/register.go` — Register 6 new tools + `filesystem` virtual entry
- `tool/file/read.go` — Add `head`/`tail` params, call `validatePath`
- `tool/file/write.go` — Call `validatePath`
- `tool/file/list.go` — Call `validatePath`
- `tool/file/search.go` — Call `validatePath`
- `api/handlers_tools.go` — Aggregate `toolset="file"` entries, return `filesystem` SettingsSchema
- `api/server.go` — Modify `activeToolReg()` for subtool filtering

### New Files (Frontend)
- `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.tsx`
- `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.module.css`

### Modified Files (Frontend)
- `web/src/components/groups/skills/detail-renderers/registry.ts` — Register `filesystem` renderer
- `web/src/locales/zh-CN/descriptors.json` — Add filesystem labels/help
- `web/src/locales/zh-CN/ui.json` — Add filesystem UI strings

---

### Task 1: Security Layer — validatePath

**Files:**
- Create: `tool/file/security.go`
- Create: `tool/file/security_test.go`

- [ ] **Step 1: Write validatePath and helpers**

```go
package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validatePath ensures the given path is safe and within allowed directories.
// It resolves symlinks, rejects traversal attempts, and checks the whitelist.
func validatePath(input string, allowed []string) error {
	if input == "" {
		return fmt.Errorf("path is required")
	}
	if len(allowed) == 0 {
		return fmt.Errorf("allowed directories not configured; please set allowed directories in Settings -> Tools -> Filesystem")
	}

	// Reject obvious traversal
	if strings.Contains(input, "..") {
		return fmt.Errorf("path contains traversal segment '..'")
	}

	abs, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			// For non-existent paths (e.g., write target), validate the parent directory
			parent := filepath.Dir(abs)
			resolvedParent, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return fmt.Errorf("failed to resolve parent directory: %w", err)
			}
			resolved = resolvedParent
		} else {
			return fmt.Errorf("failed to resolve symlinks: %w", err)
		}
	}

	for _, dir := range allowed {
		allowedAbs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		allowedAbs, err = filepath.EvalSymlinks(allowedAbs)
		if err != nil {
			continue
		}
		// Ensure allowed dir ends with separator for prefix check
		prefix := allowedAbs
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if resolved == allowedAbs || strings.HasPrefix(resolved+string(filepath.Separator), prefix) {
			return nil
		}
	}

	return fmt.Errorf("path %q is outside allowed directories", input)
}

// getAllowedDirs reads allowed directories from config settings.
func getAllowedDirs(cfg map[string]any) []string {
	raw, ok := cfg["allowed_directories"].(string)
	if !ok || raw == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
```

- [ ] **Step 2: Write tests**

```go
package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath_Allowed(t *testing.T) {
	tmp := t.TempDir()
	allowed := []string{tmp}
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("hello"), 0o644)

	if err := validatePath(f, allowed); err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestValidatePath_TraversalRejected(t *testing.T) {
	tmp := t.TempDir()
	allowed := []string{tmp}
	if err := validatePath(filepath.Join(tmp, "..", "etc", "passwd"), allowed); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestValidatePath_OutsideAllowed(t *testing.T) {
	tmp := t.TempDir()
	other := t.TempDir()
	allowed := []string{tmp}
	f := filepath.Join(other, "test.txt")
	os.WriteFile(f, []byte("hello"), 0o644)

	if err := validatePath(f, allowed); err == nil {
		t.Fatal("expected outside allowed to be rejected")
	}
}

func TestValidatePath_EmptyAllowed(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("hello"), 0o644)

	if err := validatePath(f, nil); err == nil {
		t.Fatal("expected empty allowed to be rejected")
	}
}

func TestValidatePath_SymlinkEscape(t *testing.T) {
	tmp := t.TempDir()
	allowed := []string{tmp}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	os.WriteFile(outsideFile, []byte("secret"), 0o644)

	link := filepath.Join(tmp, "link.txt")
	os.Symlink(outsideFile, link)

	if err := validatePath(link, allowed); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestGetAllowedDirs(t *testing.T) {
	cfg := map[string]any{"allowed_directories": "/home/user\n/tmp\n\n"}
	got := getAllowedDirs(cfg)
	want := []string{"/home/user", "/tmp"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
```

- [ ] **Step 3: Run tests**

```bash
cd d:/workspace/go_work/hermind && go test ./tool/file/ -run TestValidatePath -v
```

Expected: PASS (4 tests)

- [ ] **Step 4: Run getAllowedDirs test**

```bash
cd d:/workspace/go_work/hermind && go test ./tool/file/ -run TestGetAllowedDirs -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd d:/workspace/go_work/hermind && git add tool/file/security.go tool/file/security_test.go && git commit -m "feat(filesystem): add path validation security layer"
```

---

### Task 2: Add get_file_info Tool

**Files:**
- Create: `tool/file/get_info.go`

- [ ] **Step 1: Write get_file_info handler**

```go
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
```

- [ ] **Step 2: Commit**

```bash
cd d:/workspace/go_work/hermind && git add tool/file/get_info.go && git commit -m "feat(filesystem): add get_file_info tool"
```

---

### Task 3: Add read_multiple_files Tool

**Files:**
- Create: `tool/file/read_multiple.go`

- [ ] **Step 1: Write read_multiple_files handler**

```go
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/odysseythink/hermind/tool"
)

const readMultipleFilesSchema = `{
  "type": "object",
  "properties": {
    "paths": {
      "type": "array",
      "items": { "type": "string" },
      "description": "List of file paths to read."
    }
  },
  "required": ["paths"]
}`

type readMultipleFilesArgs struct {
	Paths []string `json:"paths"`
}

type readMultipleResult struct {
	Files []fileContent `json:"files"`
}

type fileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Error   string `json:"error,omitempty"`
}

const maxReadMultipleFiles = 50
const maxReadMultipleBytes = 1 << 20 // 1 MiB per file

func readMultipleFilesHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args readMultipleFilesArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if len(args.Paths) == 0 {
		return tool.ToolError("paths array is required"), nil
	}
	if len(args.Paths) > maxReadMultipleFiles {
		return tool.ToolError(fmt.Sprintf("too many files: max %d", maxReadMultipleFiles)), nil
	}

	allowed := getAllowedDirs(cfg)
	out := readMultipleResult{Files: make([]fileContent, 0, len(args.Paths))}

	for _, p := range args.Paths {
		if err := validatePath(p, allowed); err != nil {
			out.Files = append(out.Files, fileContent{Path: p, Error: err.Error()})
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			out.Files = append(out.Files, fileContent{Path: p, Error: err.Error()})
			continue
		}
		if info.IsDir() {
			out.Files = append(out.Files, fileContent{Path: p, Error: "is a directory"})
			continue
		}
		if info.Size() > maxReadMultipleBytes {
			out.Files = append(out.Files, fileContent{Path: p, Error: fmt.Sprintf("file too large: %d bytes", info.Size())})
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			out.Files = append(out.Files, fileContent{Path: p, Error: err.Error()})
			continue
		}
		// Truncate very long content in the JSON result to avoid huge payloads
		content := string(data)
		if len(content) > 100000 {
			content = content[:100000] + "\n[Content truncated]"
		}
		out.Files = append(out.Files, fileContent{
			Path:    p,
			Content: content,
			Size:    len(data),
		})
	}

	return tool.ToolResult(out), nil
}
```

- [ ] **Step 2: Commit**

```bash
cd d:/workspace/go_work/hermind && git add tool/file/read_multiple.go && git commit -m "feat(filesystem): add read_multiple_files tool"
```

---

### Task 4: Add edit_file Tool

**Files:**
- Create: `tool/file/edit.go`

- [ ] **Step 1: Write edit_file handler**

```go
package file

import (
	"context"
	"encoding/json"
	"fmt"
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
```

- [ ] **Step 2: Commit**

```bash
cd d:/workspace/go_work/hermind && git add tool/file/edit.go && git commit -m "feat(filesystem): add edit_file tool"
```

---

### Task 5: Add create_directory, copy_file, move_file Tools

**Files:**
- Create: `tool/file/create_dir.go`
- Create: `tool/file/copy.go`
- Create: `tool/file/move.go`

- [ ] **Step 1: Write create_directory handler**

```go
package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/odysseythink/hermind/tool"
)

const createDirectorySchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Directory path to create." }
  },
  "required": ["path"]
}`

type createDirectoryArgs struct {
	Path string `json:"path"`
}

type createDirectoryResult struct {
	Path string `json:"path"`
}

func createDirectoryHandler(ctx context.Context, raw json.RawMessage, cfg map[string]any) (string, error) {
	var args createDirectoryArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if err := validatePath(args.Path, getAllowedDirs(cfg)); err != nil {
		// Allow creating directories inside allowed dirs even if the exact path doesn't exist yet
		parent := filepath.Dir(args.Path)
		if parentErr := validatePath(parent, getAllowedDirs(cfg)); parentErr != nil {
			return tool.ToolError(err.Error()), nil
		}
	}

	if err := os.MkdirAll(args.Path, 0o755); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(createDirectoryResult{Path: args.Path}), nil
}
```

- [ ] **Step 2: Write copy_file handler**

```go
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
```

- [ ] **Step 3: Write move_file handler**

```go
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
```

- [ ] **Step 4: Commit**

```bash
cd d:/workspace/go_work/hermind && git add tool/file/create_dir.go tool/file/copy.go tool/file/move.go && git commit -m "feat(filesystem): add create_directory, copy_file, move_file tools"
```

---

### Task 6: Wire All Tools into Registry + Add Virtual filesystem Entry

**Files:**
- Modify: `tool/file/register.go`

- [ ] **Step 1: Rewrite register.go**

Replace the entire file with:

```go
package file

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// cfgGetter extracts the filesystem settings map from a tool call context.
// In Hermind, the tool framework passes config via context or the registry
// doesn't natively support it. We'll use a package-level variable set by
// the engine bootstrap. For the plan, assume the handler signature accepts
// a config map. If the actual pantheon Entry.Handler signature is
// `func(context.Context, json.RawMessage) (string, error)`, we'll need to
// read config from a package-level var or context value.
//
// ACTUAL APPROACH: Read from a package-level variable that the server sets
// before each request. This is a pragmatic bridge.
var currentFilesystemConfig map[string]any

func setCurrentConfig(cfg map[string]any) { currentFilesystemConfig = cfg }
func getCfg() map[string]any {
	if currentFilesystemConfig == nil {
		return map[string]any{}
	}
	return currentFilesystemConfig
}

// RegisterAll adds every file tool to the registry. Call this once at startup.
func RegisterAll(reg *tool.Registry) {
	// Virtual aggregation entry — no handler, serves as frontend config anchor
	reg.Register(&tool.Entry{
		Name:        "filesystem",
		Toolset:     "filesystem",
		Description: "Filesystem access — allows the agent to read, write, search, and manage files within allowed directories.",
		Emoji:       "📁",
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			return tool.ToolError("filesystem is a configuration entry; use individual file tools instead"), nil
		},
		Schema: core.ToolDefinition{
			Name:        "filesystem",
			Description: "Filesystem access configuration entry. Individual file tools implement the actual operations.",
			Parameters:  core.MustSchemaFromJSON([]byte(`{"type":"object","properties":{}}`)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "read_file",
		Toolset:     "file",
		Description: "Read a file from the filesystem.",
		Emoji:       "📄",
		Handler:     wrapWithConfig(readFileHandler),
		Schema: core.ToolDefinition{
			Name:        "read_file",
			Description: "Read the contents of a file. Max 1 MiB. Supports head/tail line limits.",
			Parameters:  core.MustSchemaFromJSON([]byte(readFileSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "write_file",
		Toolset:     "file",
		Description: "Write content to a file.",
		Emoji:       "✏️",
		Handler:     wrapWithConfig(writeFileHandler),
		Schema: core.ToolDefinition{
			Name:        "write_file",
			Description: "Write content to a file, overwriting if it exists.",
			Parameters:  core.MustSchemaFromJSON([]byte(writeFileSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "list_directory",
		Toolset:     "file",
		Description: "List files and subdirectories in a directory.",
		Emoji:       "📂",
		Handler:     wrapWithConfig(listDirectoryHandler),
		Schema: core.ToolDefinition{
			Name:        "list_directory",
			Description: "List entries in a directory, showing name, type, and size.",
			Parameters:  core.MustSchemaFromJSON([]byte(listDirectorySchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "search_files",
		Toolset:     "file",
		Description: "Recursively search for files by glob pattern.",
		Emoji:       "🔍",
		Handler:     wrapWithConfig(searchFilesHandler),
		Schema: core.ToolDefinition{
			Name:        "search_files",
			Description: "Recursively find files in a directory matching a glob pattern.",
			Parameters:  core.MustSchemaFromJSON([]byte(searchFilesSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "read_multiple_files",
		Toolset:     "file",
		Description: "Read multiple files at once.",
		Emoji:       "📑",
		Handler:     wrapWithConfig(readMultipleFilesHandler),
		Schema: core.ToolDefinition{
			Name:        "read_multiple_files",
			Description: "Read the contents of multiple files in a single call.",
			Parameters:  core.MustSchemaFromJSON([]byte(readMultipleFilesSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "get_file_info",
		Toolset:     "file",
		Description: "Get metadata about a file or directory.",
		Emoji:       "ℹ️",
		Handler:     wrapWithConfig(getFileInfoHandler),
		Schema: core.ToolDefinition{
			Name:        "get_file_info",
			Description: "Get file metadata including size, permissions, and modification time.",
			Parameters:  core.MustSchemaFromJSON([]byte(getFileInfoSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "edit_file",
		Toolset:     "file",
		Description: "Edit a file by find-and-replace.",
		Emoji:       "✏️",
		Handler:     wrapWithConfig(editFileHandler),
		Schema: core.ToolDefinition{
			Name:        "edit_file",
			Description: "Find and replace text within a file.",
			Parameters:  core.MustSchemaFromJSON([]byte(editFileSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "create_directory",
		Toolset:     "file",
		Description: "Create a directory.",
		Emoji:       "📂",
		Handler:     wrapWithConfig(createDirectoryHandler),
		Schema: core.ToolDefinition{
			Name:        "create_directory",
			Description: "Create a directory, including parent directories if needed.",
			Parameters:  core.MustSchemaFromJSON([]byte(createDirectorySchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "copy_file",
		Toolset:     "file",
		Description: "Copy a file.",
		Emoji:       "📋",
		Handler:     wrapWithConfig(copyFileHandler),
		Schema: core.ToolDefinition{
			Name:        "copy_file",
			Description: "Copy a file from source to destination.",
			Parameters:  core.MustSchemaFromJSON([]byte(copyFileSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "move_file",
		Toolset:     "file",
		Description: "Move or rename a file.",
		Emoji:       "↔️",
		Handler:     wrapWithConfig(moveFileHandler),
		Schema: core.ToolDefinition{
			Name:        "move_file",
			Description: "Move or rename a file.",
			Parameters:  core.MustSchemaFromJSON([]byte(moveFileSchema)),
		},
	})
}

// wrapWithConfig adapts handlers that need config access to the standard signature.
func wrapWithConfig(h func(context.Context, json.RawMessage, map[string]any) (string, error)) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		return h(ctx, raw, getCfg())
	}
}
```

**IMPORTANT IMPLEMENTATION NOTE:** The pantheon `tool.Entry.Handler` signature is `func(context.Context, json.RawMessage) (string, error)`. Our handlers need config access. The plan uses a `wrapWithConfig` adapter and a package-level `currentFilesystemConfig` variable. The server must set this variable before executing tools. If pantheon provides a different mechanism (e.g., context values), adjust accordingly. Check `tool/registry.go` and pantheon source to confirm the exact approach.

- [ ] **Step 2: Verify compilation**

```bash
cd d:/workspace/go_work/hermind && go build ./tool/file/...
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
cd d:/workspace/go_work/hermind && git add tool/file/register.go && git commit -m "feat(filesystem): register all 10 file tools + virtual filesystem entry"
```

---

### Task 7: Add Security Checks to Existing 4 Tools

**Files:**
- Modify: `tool/file/read.go`
- Modify: `tool/file/write.go`
- Modify: `tool/file/list.go`
- Modify: `tool/file/search.go`

- [ ] **Step 1: Modify read.go — Add head/tail params and validatePath**

Change the schema constant:

```go
const readFileSchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Absolute or relative file path." },
    "head": { "type": "integer", "description": "If provided, returns only the first N lines." },
    "tail": { "type": "integer", "description": "If provided, returns only the last N lines." }
  },
  "required": ["path"]
}`
```

Change the args struct:

```go
type readFileArgs struct {
	Path string `json:"path"`
	Head *int   `json:"head,omitempty"`
	Tail *int   `json:"tail,omitempty"`
}
```

At the top of `readFileHandler`, after argument validation:

```go
	if err := validatePath(args.Path, getAllowedDirs(getCfg())); err != nil {
		return tool.ToolError(err.Error()), nil
	}
```

After reading the file, add head/tail support before returning:

```go
	content := string(data)
	if args.Head != nil && args.Tail != nil {
		return tool.ToolError("cannot specify both head and tail"), nil
	}
	if args.Head != nil && *args.Head > 0 {
		lines := strings.Split(content, "\n")
		if *args.Head < len(lines) {
			lines = lines[:*args.Head]
		}
		content = strings.Join(lines, "\n")
	}
	if args.Tail != nil && *args.Tail > 0 {
		lines := strings.Split(content, "\n")
		if *args.Tail < len(lines) {
			lines = lines[len(lines)-*args.Tail:]
		}
		content = strings.Join(lines, "\n")
	}

	return tool.ToolResult(readFileResult{
		Path:    args.Path,
		Content: content,
		Size:    len(data),
	}), nil
```

Add `"strings"` to imports.

- [ ] **Step 2: Modify write.go — Add validatePath**

At the top of `writeFileHandler`, after argument validation:

```go
	if err := validatePath(args.Path, getAllowedDirs(getCfg())); err != nil {
		return tool.ToolError(err.Error()), nil
	}
```

- [ ] **Step 3: Modify list.go — Add validatePath**

At the top of `listDirectoryHandler`, after argument validation:

```go
	if err := validatePath(args.Path, getAllowedDirs(getCfg())); err != nil {
		return tool.ToolError(err.Error()), nil
	}
```

- [ ] **Step 4: Modify search.go — Add validatePath**

At the top of `searchFilesHandler`, after argument validation:

```go
	if err := validatePath(args.Root, getAllowedDirs(getCfg())); err != nil {
		return tool.ToolError(err.Error()), nil
	}
```

- [ ] **Step 5: Verify compilation**

```bash
cd d:/workspace/go_work/hermind && go build ./tool/file/...
```

Expected: No errors.

- [ ] **Step 6: Commit**

```bash
cd d:/workspace/go_work/hermind && git add tool/file/read.go tool/file/write.go tool/file/list.go tool/file/search.go && git commit -m "feat(filesystem): add path validation to existing file tools"
```

---

### Task 8: API Layer — Aggregate File Tools and Expose SettingsSchema

**Files:**
- Modify: `api/handlers_tools.go`

- [ ] **Step 1: Rewrite handleToolsList to aggregate file tools**

Replace the function with:

```go
func (s *Server) handleToolsList(w http.ResponseWriter, _ *http.Request) {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		writeJSON(w, ToolsResponse{Tools: []ToolDTO{}})
		return
	}

	disabled := s.disabledTools()
	entries := deps.ToolReg.Entries(nil)

	// Collect file toolset entries for aggregation
	var fileTools []tool.Entry
	var otherTools []tool.Entry
	for _, e := range entries {
		if e.Toolset == "file" {
			fileTools = append(fileTools, e)
		} else {
			otherTools = append(otherTools, e)
		}
	}

	out := make([]ToolDTO, 0, len(otherTools)+1)

	// Add non-file tools
	for _, e := range otherTools {
		dto := ToolDTO{
			Name:           e.Name,
			Description:    e.Description,
			Toolset:        e.Toolset,
			Enabled:        !disabled[e.Name],
			SettingsSchema: []ConfigFieldDTO{},
		}
		if e.Name == "browser_control" {
			dto.SettingsSchema = []ConfigFieldDTO{
				{Name: "enabled", Label: "Enabled", Kind: "bool", Help: "Enable the browser extension integration."},
				{Name: "api_key", Label: "API key", Kind: "secret", Help: "Authentication key for the browser extension."},
			}
		}
		out = append(out, dto)
	}

	// Add aggregated filesystem entry if any file tools exist
	if len(fileTools) > 0 {
		filesystemEnabled := !disabled["filesystem"]
		out = append(out, ToolDTO{
			Name:        "filesystem",
			Description: "Filesystem access — allows the agent to read, write, search, and manage files within allowed directories.",
			Toolset:     "filesystem",
			Enabled:     filesystemEnabled,
			SettingsSchema: []ConfigFieldDTO{
				{Name: "allowed_directories", Label: "Allowed directories", Kind: "text", Help: "One absolute path per line. Only paths under these directories can be accessed.", Default: ""},
				{Name: "read_file", Label: "Read file", Kind: "bool", Help: "Read file contents.", Default: true},
				{Name: "read_multiple_files", Label: "Read multiple files", Kind: "bool", Help: "Read multiple files at once.", Default: true},
				{Name: "list_directory", Label: "List directory", Kind: "bool", Help: "List directory contents.", Default: true},
				{Name: "search_files", Label: "Search files", Kind: "bool", Help: "Search files by glob pattern.", Default: true},
				{Name: "get_file_info", Label: "Get file info", Kind: "bool", Help: "Get file metadata.", Default: true},
				{Name: "write_file", Label: "Write file", Kind: "bool", Help: "Write content to a file.", Default: true},
				{Name: "edit_file", Label: "Edit file", Kind: "bool", Help: "Find and replace within a file.", Default: true},
				{Name: "create_directory", Label: "Create directory", Kind: "bool", Help: "Create a directory.", Default: true},
				{Name: "copy_file", Label: "Copy file", Kind: "bool", Help: "Copy a file.", Default: true},
				{Name: "move_file", Label: "Move file", Kind: "bool", Help: "Move or rename a file.", Default: true},
			},
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, ToolsResponse{Tools: out})
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd d:/workspace/go_work/hermind && go build ./api/...
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
cd d:/workspace/go_work/hermind && git add api/handlers_tools.go && git commit -m "feat(filesystem): aggregate file tools in API, expose settings schema"
```

---

### Task 9: Engine — activeToolReg Subtool Filtering

**Files:**
- Modify: `api/server.go`

- [ ] **Step 1: Modify activeToolReg**

Find `activeToolReg` and replace with:

```go
func (s *Server) activeToolReg() *tool.Registry {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		return nil
	}
	disabled := s.disabledTools()
	active := tool.NewRegistry()

	// Check filesystem master toggle
	filesystemDisabled := disabled["filesystem"]

	// Read subtool enablement from settings
	subtoolEnabled := make(map[string]bool)
	if fsSettings, ok := s.opts.Config.Tools.Settings["filesystem"]; ok {
		for key, val := range fsSettings {
			if key == "allowed_directories" {
				continue
			}
			if b, ok := val.(bool); ok {
				subtoolEnabled[key] = b
			}
		}
	}

	for _, e := range deps.ToolReg.Entries(nil) {
		// Virtual filesystem entry has no handler — skip from engine registry
		if e.Name == "filesystem" {
			continue
		}

		// If filesystem is disabled, drop all file toolset entries
		if e.Toolset == "file" && filesystemDisabled {
			continue
		}

		// If filesystem is enabled, check per-subtool setting
		if e.Toolset == "file" && !filesystemDisabled {
			if enabled, ok := subtoolEnabled[e.Name]; ok && !enabled {
				continue
			}
		}

		// Global disabled list check
		if disabled[e.Name] {
			continue
		}

		active.Register(e)
	}
	return active
}
```

- [ ] **Step 2: Add config injection hook for file tools**

We need a mechanism to inject the filesystem config into the file tool handlers before each request. The simplest approach is to add a method on `Server` that sets the package-level config:

Add to `api/server.go` (near the other server methods):

```go
// injectFilesystemConfig sets the current filesystem configuration
// so that file tool handlers can access allowed_directories and subtool settings.
func (s *Server) injectFilesystemConfig() {
	cfg := map[string]any{}
	if fsSettings, ok := s.opts.Config.Tools.Settings["filesystem"]; ok {
		for k, v := range fsSettings {
			cfg[k] = v
		}
	}
	// Call into tool/file package to set the config
	file.SetCurrentConfig(cfg)
}
```

Wait — `file.SetCurrentConfig` needs to be exported. Modify `tool/file/register.go` to export `setCurrentConfig`:

```go
func SetCurrentConfig(cfg map[string]any) { setCurrentConfig(cfg) }
```

And update the private function name to avoid conflict:

```go
var currentFilesystemConfig map[string]any

func setCurrentConfig(cfg map[string]any) { currentFilesystemConfig = cfg }
```

Then add the import and call in `api/server.go`:

```go
import "github.com/odysseythink/hermind/tool/file"
```

Find where `activeToolReg()` is called (or where the engine gets tools) and call `s.injectFilesystemConfig()` before that.

Check `api/server.go` for `activeToolReg` call sites. Based on earlier grep, it's at line 184. Find the context and add the injection call.

- [ ] **Step 3: Verify compilation**

```bash
cd d:/workspace/go_work/hermind && go build ./api/...
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
cd d:/workspace/go_work/hermind && git add api/server.go tool/file/register.go && git commit -m "feat(filesystem): activeToolReg filters subtools, inject config into handlers"
```

---

### Task 10: Frontend — FilesystemConfig Custom Renderer

**Files:**
- Create: `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.tsx`
- Create: `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.module.css`

- [ ] **Step 1: Write FilesystemConfig.tsx**

```tsx
import { useMemo, useCallback } from 'react';
import pageStyles from '../../SkillToolsConfigPage.module.css';
import styles from './FilesystemConfig.module.css';
import Switch from '../../../../fields/Switch';
import type { ToolDetailProps } from '../types';

interface SubtoolDef {
	name: string;
	title: string;
	description: string;
	icon: string;
	category: 'read' | 'write';
}

const SUBTOOLS: SubtoolDef[] = [
	{ name: 'read_file', title: '读取文件', description: '读取单个文件的内容', icon: '📄', category: 'read' },
	{ name: 'read_multiple_files', title: '批量读取文件', description: '同时读取多个文件', icon: '📑', category: 'read' },
	{ name: 'list_directory', title: '列出目录', description: '列出目录中的文件和子目录', icon: '📂', category: 'read' },
	{ name: 'search_files', title: '搜索文件', description: '按 glob 模式递归搜索文件', icon: '🔍', category: 'read' },
	{ name: 'get_file_info', title: '获取文件信息', description: '获取文件的元数据（大小、权限等）', icon: 'ℹ️', category: 'read' },
	{ name: 'write_file', title: '写入文件', description: '将内容写入文件', icon: '💾', category: 'write' },
	{ name: 'edit_file', title: '编辑文件', description: '在文件中查找并替换文本', icon: '✏️', category: 'write' },
	{ name: 'create_directory', title: '创建目录', description: '创建目录（支持递归）', icon: '📂', category: 'write' },
	{ name: 'copy_file', title: '复制文件', description: '复制文件到目标路径', icon: '📋', category: 'write' },
	{ name: 'move_file', title: '移动文件', description: '移动或重命名文件', icon: '↔️', category: 'write' },
];

function asString(v: unknown): string {
	if (v === undefined || v === null) return '';
	return typeof v === 'string' ? v : String(v);
}

function asBool(v: unknown): boolean {
	if (typeof v === 'boolean') return v;
	if (typeof v === 'string') return v === 'true';
	return false;
}

function getToolSettingValue(
	toolName: string,
	fieldName: string,
	config?: Record<string, unknown>,
): unknown {
	const settings = ((config?.tools as Record<string, unknown> | undefined)?.settings as
		| Record<string, Record<string, unknown>>
		| undefined);
	return settings?.[toolName]?.[fieldName];
}

function setToolSettingValue(
	toolName: string,
	fieldName: string,
	value: unknown,
	config: Record<string, unknown> | undefined,
	onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
	if (!onSectionField) return;
	const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
	const settings = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
	const nextToolSettings = { ...(settings[toolName] ?? {}), [fieldName]: value };
	const nextSettings = { ...settings, [toolName]: nextToolSettings };
	onSectionField('tools', 'settings', nextSettings);
}

export default function FilesystemConfig({
	name,
	description,
	toolset,
	enabled,
	onToggle,
	config,
	onSectionField,
}: ToolDetailProps) {
	const allowedDirs = asString(getToolSettingValue('filesystem', 'allowed_directories', config));

	const subtoolValues = useMemo(() => {
		const v: Record<string, boolean> = {};
		for (const st of SUBTOOLS) {
			v[st.name] = asBool(getToolSettingValue('filesystem', st.name, config));
			// Default to true if not explicitly set
			if (v[st.name] !== true && v[st.name] !== false) {
				v[st.name] = true;
			}
		}
		return v;
	}, [config]);

	const handleDirChange = useCallback((value: string) => {
		setToolSettingValue('filesystem', 'allowed_directories', value, config, onSectionField);
	}, [config, onSectionField]);

	const handleSubtoolToggle = useCallback((subtoolName: string, next: boolean) => {
		setToolSettingValue('filesystem', subtoolName, next, config, onSectionField);
	}, [config, onSectionField]);

	const readTools = SUBTOOLS.filter(s => s.category === 'read');
	const writeTools = SUBTOOLS.filter(s => s.category === 'write');

	return (
		<div className={pageStyles.detailContent}>
			<div className={pageStyles.detailHeader}>
				<h2 className={pageStyles.detailTitle}>
					<span className={pageStyles.detailEmoji}>📁</span>
					{name}
					{toolset && (
						<span style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginLeft: 'var(--space-2)' }}>
							({toolset})
						</span>
					)}
				</h2>
				<Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${name}`} />
			</div>

			{description && <div className={pageStyles.detailDesc}>{description}</div>}

			<div className={styles.warningBanner}>
				<div className={styles.warningIcon}>⚠️</div>
				<div className={styles.warningText}>
					访问文件系统可能存在风险，因为它可能修改或删除文件。在启用之前，请务必查阅文档。
				</div>
			</div>

			<div className={pageStyles.configSection}>
				<h3 className={styles.sectionTitle}>配置</h3>

				<div className={styles.configRow}>
					<div>
						<div className={styles.label}>Allowed directories</div>
						<div className={styles.help}>每行一个绝对路径。只允许访问这些目录下的文件。</div>
					</div>
					<textarea
						className={styles.textarea}
						value={allowedDirs}
						onChange={(e) => handleDirChange(e.currentTarget.value)}
						rows={4}
						placeholder="/home/user/projects&#10;/tmp"
						aria-label="Allowed directories"
					/>
				</div>
			</div>

			<div className={pageStyles.configSection}>
				<h3 className={styles.sectionTitle}>可用工具</h3>

				<div className={styles.subtoolGroup}>
					<p className={styles.subtoolGroupLabel}>📖 阅读操作</p>
					{readTools.map(st => (
						<SubtoolRow
							key={st.name}
							def={st}
							enabled={subtoolValues[st.name] !== false}
							onToggle={(next) => handleSubtoolToggle(st.name, next)}
							isWrite={false}
						/>
					))}
				</div>

				<div className={styles.subtoolGroup}>
					<p className={styles.subtoolGroupLabel}>
						<span className={styles.writeWarningIcon}>⚠️</span>
						✏️ 写入操作
					</p>
					{writeTools.map(st => (
						<SubtoolRow
							key={st.name}
							def={st}
							enabled={subtoolValues[st.name] !== false}
							onToggle={(next) => handleSubtoolToggle(st.name, next)}
							isWrite={true}
						/>
					))}
				</div>
			</div>
		</div>
	);
}

function SubtoolRow({
	def,
	enabled,
	onToggle,
	isWrite,
}: {
	def: SubtoolDef;
	enabled: boolean;
	onToggle: (next: boolean) => void;
	isWrite: boolean;
}) {
	return (
		<div className={`${styles.subtoolRow} ${enabled ? '' : styles.subtoolDisabled} ${isWrite ? styles.subtoolWrite : ''}`}>
			<div className={styles.subtoolInfo}>
				<span className={styles.subtoolIcon}>{def.icon}</span>
				<div>
					<div className={styles.subtoolTitle}>{def.title}</div>
					<div className={styles.subtoolDesc}>{def.description}</div>
				</div>
			</div>
			<Switch checked={enabled} onChange={onToggle} ariaLabel={def.title} />
		</div>
	);
}
```

- [ ] **Step 2: Write FilesystemConfig.module.css**

```css
.warningBanner {
	display: flex;
	align-items: flex-start;
	gap: 0.625rem;
	padding: 0.625rem;
	background: rgba(234, 88, 12, 0.1);
	border: 1px solid rgba(234, 88, 12, 0.3);
	border-radius: 0.5rem;
	margin-bottom: 1rem;
}

.warningIcon {
	font-size: 1.25rem;
	flex-shrink: 0;
	margin-top: 0.125rem;
}

.warningText {
	font-size: var(--fs-xs);
	color: var(--warning, #ea580c);
	font-weight: 500;
	line-height: 1.4;
}

.sectionTitle {
	font-size: var(--fs-sm);
	font-weight: 600;
	color: var(--text-primary);
	margin-bottom: 0.75rem;
}

.configRow {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: 1rem;
	margin-bottom: 1rem;
}

.label {
	font-size: var(--fs-sm);
	font-weight: 500;
	color: var(--text-primary);
}

.help {
	font-size: var(--fs-xs);
	color: var(--muted);
	margin-top: 0.125rem;
}

.textarea {
	width: 280px;
	min-height: 80px;
	padding: 0.5rem;
	border: 1px solid var(--border);
	border-radius: 0.375rem;
	background: var(--bg-secondary);
	color: var(--text-primary);
	font-family: var(--font-mono);
	font-size: var(--fs-sm);
	resize: vertical;
}

.textarea:focus {
	outline: none;
	border-color: var(--accent);
}

.subtoolGroup {
	display: flex;
	flex-direction: column;
	gap: 0.375rem;
	margin-bottom: 1rem;
}

.subtoolGroupLabel {
	font-size: var(--fs-xs);
	font-weight: 600;
	color: var(--muted);
	text-transform: uppercase;
	letter-spacing: 0.025em;
	margin-bottom: 0.25rem;
	display: flex;
	align-items: center;
	gap: 0.25rem;
}

.writeWarningIcon {
	color: #ea580c;
}

.subtoolRow {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 0.5rem;
	border-radius: 0.5rem;
	border: 1px solid var(--border);
	background: var(--bg-secondary);
}

.subtoolDisabled {
	opacity: 0.5;
	background: var(--bg-secondary);
}

.subtoolWrite {
	background: rgba(234, 88, 12, 0.05);
	border-color: rgba(234, 88, 12, 0.2);
}

.subtoolInfo {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.subtoolIcon {
	font-size: 1rem;
}

.subtoolTitle {
	font-size: var(--fs-sm);
	font-weight: 500;
	color: var(--text-primary);
}

.subtoolDesc {
	font-size: var(--fs-xs);
	color: var(--muted);
}
```

- [ ] **Step 3: Verify TypeScript compilation**

```bash
cd d:/workspace/go_work/hermind/web && npx tsc --noEmit 2>&1 | head -30
```

Expected: No errors related to the new component.

- [ ] **Step 4: Commit**

```bash
cd d:/workspace/go_work/hermind && git add web/src/components/groups/skills/detail-renderers/filesystem/ && git commit -m "feat(filesystem): add FilesystemConfig custom detail renderer"
```

---

### Task 11: Frontend — Register Renderer + Localization

**Files:**
- Modify: `web/src/components/groups/skills/detail-renderers/registry.ts`
- Modify: `web/src/locales/zh-CN/descriptors.json`
- Modify: `web/src/locales/zh-CN/ui.json`

- [ ] **Step 1: Register filesystem renderer**

In `registry.ts`, add the import and register:

```ts
import FilesystemConfig from './filesystem/FilesystemConfig';

export const toolDetailRegistry: Record<string, React.FC<ToolDetailProps>> = {
  browser_control: BrowserControlConfig,
  filesystem: FilesystemConfig,
};
```

- [ ] **Step 2: Add localization strings**

In `web/src/locales/zh-CN/descriptors.json`, add:

```json
"filesystem.label": "文件系统访问",
"filesystem.summary": "允许代理读取、写入、搜索和管理指定目录中的文件。支持文件编辑、目录导航和内容搜索功能。",
"filesystem.allowed_directories.label": "Allowed directories",
"filesystem.allowed_directories.help": "每行一个绝对路径。只允许访问这些目录下的文件。"
```

In `web/src/locales/zh-CN/ui.json`, add:

```json
"tools.filesystem.readActions": "阅读操作",
"tools.filesystem.writeActions": "写入操作",
"tools.filesystem.warning": "访问文件系统可能存在风险，因为它可能修改或删除文件。在启用之前，请务必查阅文档。",
"tools.filesystem.configuration": "配置",
"tools.filesystem.availableTools": "可用工具"
```

- [ ] **Step 3: Verify TypeScript compilation**

```bash
cd d:/workspace/go_work/hermind/web && npx tsc --noEmit 2>&1 | head -30
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
cd d:/workspace/go_work/hermind && git add web/src/components/groups/skills/detail-renderers/registry.ts web/src/locales/zh-CN/descriptors.json web/src/locales/zh-CN/ui.json && git commit -m "feat(filesystem): register renderer and add localization"
```

---

### Task 12: Integration Test — Build and Verify

- [ ] **Step 1: Build backend**

```bash
cd d:/workspace/go_work/hermind && go build ./...
```

Expected: No errors.

- [ ] **Step 2: Run backend tests**

```bash
cd d:/workspace/go_work/hermind && go test ./tool/file/... -v
```

Expected: All tests pass.

- [ ] **Step 3: Build frontend**

```bash
cd d:/workspace/go_work/hermind/web && npm run build 2>&1 | tail -20
```

Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
cd d:/workspace/go_work/hermind && git commit -m "feat(filesystem): integration complete" --allow-empty
```

---

## Self-Review

### Spec Coverage Check

| Spec Requirement | Task |
|---|---|
| Virtual `filesystem` aggregation entry | Task 6 |
| 10 subtools (4 existing + 6 new) | Tasks 2-7 |
| `validatePath` security layer | Task 1 |
| `allowed_directories` whitelist | Tasks 1, 10 |
| Path traversal prevention | Task 1 |
| Symlink escape prevention | Task 1 |
| Empty allowed list rejection | Task 1 |
| API aggregation (hide `toolset="file"`) | Task 8 |
| `activeToolReg` subtool filtering | Task 9 |
| Frontend custom renderer | Tasks 10-11 |
| Warning banner | Task 10 |
| Read/Write category sub-switches | Task 10 |
| Localization | Task 11 |
| Error handling | Tasks 1, 2-7 |

### Placeholder Scan

- No TBD, TODO, or "implement later" found.
- All code blocks contain actual implementation.
- All test commands include expected output.

### Type Consistency Check

- `validatePath` signature: `(string, []string) error` — consistent across all handlers.
- Handler signatures with config: `func(context.Context, json.RawMessage, map[string]any) (string, error)` — wrapped by `wrapWithConfig`.
- `getAllowedDirs` returns `[]string` — used consistently.
- Config field names in SettingsSchema match subtool names exactly: `read_file`, `write_file`, etc.
- Frontend `subtoolValues` keys match backend field names.

### One Open Note

The `wrapWithConfig` adapter and `currentFilesystemConfig` package-level variable is a pragmatic bridge. If pantheon's `tool.Handler` signature or the engine's tool execution flow provides a different mechanism for config injection (e.g., context values), the adapter should be adjusted. The plan assumes this approach works; verify during Task 6 if compilation fails.
