package file

import (
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
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
		Schema: core.ToolDefinition{
			Name:        "read_file",
			Description: "Read the contents of a file. Max 1 MiB.",
			Parameters:  core.MustSchemaFromJSON([]byte(readFileSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "write_file",
		Toolset:     "file",
		Description: "Write content to a file.",
		Emoji:       "✏️",
		Handler:     writeFileHandler,
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
		Emoji:       "📁",
		Handler:     listDirectoryHandler,
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
		Handler:     searchFilesHandler,
		Schema: core.ToolDefinition{
			Name:        "search_files",
			Description: "Recursively find files in a directory matching a glob pattern.",
			Parameters:  core.MustSchemaFromJSON([]byte(searchFilesSchema)),
		},
	})
}
