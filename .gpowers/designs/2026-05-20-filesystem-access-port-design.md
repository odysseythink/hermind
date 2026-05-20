# Filesystem Access Feature Port — Design Spec

**Date:** 2026-05-20
**Source:** AnythingLLM v1.12.1 filesystem agent skill
**Target:** Hermind Go backend + React/Vite Web UI

---

## Goal

Port AnythingLLM's "Filesystem Access" agent skill into Hermind. The feature allows the LLM agent to read, write, search, and manage files within allowed directories, with per-subtool enable/disable controls and an allowed-directories security whitelist.

## Architecture

### High-Level Flow

```
User opens Settings → Skills → Tools list
  → Clicks "filesystem" item
    → Custom FilesystemConfig detail panel renders
      ├── Master toggle (enable/disable entire filesystem)
      ├── Warning banner (filesystem access is risky)
      ├── Allowed Directories input
      ├── Read Operations sub-switches (5 tools)
      └── Write Operations sub-switches (5 tools)
```

### Backend Design

#### Virtual Aggregation Entry

A virtual tool named `filesystem` is registered with `Toolset: "filesystem"`. It has no handler — it serves purely as the frontend aggregation point.

All actual file operations are implemented as independent `tool.Entry` instances with `Toolset: "file"`. The API layer (`handleToolsList`) hides individual `toolset="file"` entries and exposes only the single `filesystem` entry with a populated `SettingsSchema`.

#### 10 Subtools

| Tool Name | Category | Description |
|-----------|----------|-------------|
| `read_file` | Read | Read file contents. Supports `head` and `tail` line limits. |
| `read_multiple_files` | Read | Read multiple files in a single call. |
| `list_directory` | Read | List files and subdirectories with name, type, and size. |
| `search_files` | Read | Recursively find files matching a glob pattern. |
| `get_file_info` | Read | Get file metadata (size, mod time, permissions, is_dir). |
| `write_file` | Write | Write content to a file, overwriting if exists. |
| `edit_file` | Write | Find-and-replace or line-based edit within a file. |
| `create_directory` | Write | Create a directory (recursive if needed). |
| `copy_file` | Write | Copy a file from source to destination. |
| `move_file` | Write | Move or rename a file. |

**Note:** `read_file`, `write_file`, `list_directory`, `search_files` already exist in `tool/file/`. The other 6 are new.

#### Security Layer

Every file operation handler calls `validatePath(path string, allowed []string) error` before doing any I/O.

`validatePath` performs:
1. **Path traversal check:** Rejects paths containing `..` segments.
2. **Symlink resolution:** Calls `filepath.EvalSymlinks` and re-validates the resolved path.
3. **Allowed directory check:** Ensures the resolved absolute path is under at least one entry in `allowed_directories`.
4. **Empty allowed list fallback:** If `allowed_directories` is empty, all file operations are rejected with an error prompting the user to configure allowed directories.

The allowed directories list is stored in `config.tools.settings.filesystem.allowed_directories` as a `[]string`.

#### Tool Enable/Disable Logic

`activeToolReg()` is modified to:
1. If `filesystem` is in `tools.disabled`, filter out ALL `toolset="file"` entries.
2. Otherwise, for each `toolset="file"` entry, check `config.tools.settings.filesystem.subtool_enabled[tool_name]`. If false, skip registration.
3. Non-file tools are unaffected.

#### Settings Schema (API DTO)

`handleToolsList` returns the following `SettingsSchema` for the `filesystem` entry:

```
allowed_directories  (kind: text, multiline-like, help: "One path per line")
read_file            (kind: bool, default: true)
read_multiple_files  (kind: bool, default: true)
list_directory       (kind: bool, default: true)
search_files         (kind: bool, default: true)
get_file_info        (kind: bool, default: true)
write_file           (kind: bool, default: true)
edit_file            (kind: bool, default: true)
create_directory     (kind: bool, default: true)
copy_file            (kind: bool, default: true)
move_file            (kind: bool, default: true)
```

Because the `ConfigFieldDTO` kind system does not have a native multiline text type, `allowed_directories` is rendered as a `<textarea>` on the frontend (overridden in the custom renderer).

### Frontend Design

#### FilesystemConfig Custom Renderer

A new React component `FilesystemConfig` is registered in `toolDetailRegistry` under the key `"filesystem"`.

The component renders:
1. **Header row:** Tool name + master toggle Switch.
2. **Warning banner:** Orange-styled alert box with a warning icon and text: "访问文件系统可能存在风险，因为它可能修改或删除文件。在启用之前，请务必查阅文档。"
3. **Description:** Brief text explaining what filesystem access does.
4. **Allowed Directories:** Textarea input for entering one path per line.
5. **Read Operations section:** 5 subtool rows, each with an icon, title, description, and toggle switch. Styled with neutral background.
6. **Write Operations section:** 5 subtool rows, each with an icon, title, description, and toggle switch. Styled with orange-tinted background to indicate destructive potential.

#### State Management

The component reads/writes via `props.onSectionField('tools', 'settings', {...})`:
- `config.tools.settings.filesystem.allowed_directories` → string (newline-separated, converted to/from `[]string`)
- `config.tools.settings.filesystem.subtool_enabled` → `Record<string, boolean>`

The master toggle uses `props.onToggle` which controls whether `filesystem` is in `tools.disabled`.

#### Icons

Read operations use neutral icons (📄, 📑, 📁, 🔍, ℹ️). Write operations use action-oriented icons (💾, ✏️, 📂, 📋, ↔️). These are rendered as emoji for simplicity and zero dependency cost.

### Files Changed

#### New Files

- `tool/file/read_multiple.go` — `read_multiple_files` handler
- `tool/file/get_info.go` — `get_file_info` handler
- `tool/file/edit.go` — `edit_file` handler
- `tool/file/create_dir.go` — `create_directory` handler
- `tool/file/copy.go` — `copy_file` handler
- `tool/file/move.go` — `move_file` handler
- `tool/file/security.go` — `validatePath` and helpers
- `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.tsx`
- `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.module.css`

#### Modified Files

- `tool/file/register.go` — Register new tools + filesystem virtual entry
- `tool/file/read.go` — Add `head`/`tail` params, add `validatePath` call
- `tool/file/write.go` — Add `validatePath` call
- `tool/file/list.go` — Add `validatePath` call
- `tool/file/search.go` — Add `validatePath` call
- `api/handlers_tools.go` — Aggregate toolset="file", add filesystem SettingsSchema
- `api/server.go` — Modify `activeToolReg` for subtool filtering
- `web/src/components/groups/skills/detail-renderers/registry.ts` — Register filesystem renderer
- `web/src/locales/zh-CN/descriptors.json` — Add filesystem labels/help text
- `web/src/locales/zh-CN/ui.json` — Add filesystem UI strings

### Error Handling

- **Path validation failure:** Tool returns a `ToolError` with a clear message indicating the path is outside allowed directories or contains traversal segments.
- **File not found:** Standard `os` error propagated through `ToolError`.
- **Permission denied:** Standard `os` error propagated through `ToolError`.
- **Empty allowed directories:** All file tools reject with `"Allowed directories not configured. Please set allowed directories in Settings → Tools → Filesystem."`

### Testing Strategy

- Unit tests for `validatePath` covering traversal, symlinks, allowed directory boundaries.
- Unit tests for each new tool handler.
- Frontend component tests for `FilesystemConfig` (toggle states, textarea updates).

### Scope Exclusions (YAGNI)

- File upload/download via HTTP
- Binary file editing
- File watching / inotify
- Cross-platform ACL handling
- Image file auto-attachment (AnythingLLM feature, out of scope)

---

## Open Questions

None at design time — all clarified with user.
