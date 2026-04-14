package file

import (
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
)

// RegisterAll adds every file tool (read_file, write_file, list_directory,
// search_files) to the registry. Call this once at startup.
func RegisterAll(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "read_file",
		Toolset:     "file",
		Description: "Read a file from the filesystem.",
		Emoji:       "📄",
		Handler:     readFileHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "read_file",
				Description: "Read the contents of a file. Max 1 MiB.",
				Parameters:  json.RawMessage(readFileSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "write_file",
		Toolset:     "file",
		Description: "Write content to a file.",
		Emoji:       "✏️",
		Handler:     writeFileHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "write_file",
				Description: "Write content to a file, overwriting if it exists.",
				Parameters:  json.RawMessage(writeFileSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "list_directory",
		Toolset:     "file",
		Description: "List files and subdirectories in a directory.",
		Emoji:       "📁",
		Handler:     listDirectoryHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "list_directory",
				Description: "List entries in a directory, showing name, type, and size.",
				Parameters:  json.RawMessage(listDirectorySchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "search_files",
		Toolset:     "file",
		Description: "Recursively search for files by glob pattern.",
		Emoji:       "🔍",
		Handler:     searchFilesHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "search_files",
				Description: "Recursively find files in a directory matching a glob pattern.",
				Parameters:  json.RawMessage(searchFilesSchema),
			},
		},
	})
}
