package file

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// currentFilesystemConfig holds the active filesystem settings for the current
// request. The server sets this before dispatching file tools.
var currentFilesystemConfig map[string]any

func setCurrentConfig(cfg map[string]any) { currentFilesystemConfig = cfg }

// SetCurrentConfig exports the config setter for use by the API server.
func SetCurrentConfig(cfg map[string]any) { setCurrentConfig(cfg) }

func getCfg() map[string]any {
	if currentFilesystemConfig == nil {
		return map[string]any{}
	}
	return currentFilesystemConfig
}

// RegisterAll adds every file tool to the registry. Call this once at startup.
func RegisterAll(reg *tool.Registry) {
	// Virtual aggregation entry — no real handler, serves as frontend config anchor
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
