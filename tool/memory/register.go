// tool/memory/register.go
package memory

import (
	"encoding/json"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// RegisterAll registers memory_save, memory_search, and memory_delete into
// the given registry. The storage argument must be non-nil — if memory
// functionality isn't wanted, don't call this function.
func RegisterAll(reg *tool.Registry, store storage.Storage) {
	reg.Register(&tool.Entry{
		Name:        "memory_save",
		Toolset:     "memory",
		Description: "Save a memory the agent should remember across conversations.",
		Emoji:       "🧠",
		Handler:     newMemorySaveHandler(store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "memory_save",
				Description: "Save a fact, preference, or instruction to persistent memory.",
				Parameters:  json.RawMessage(memorySaveSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "memory_search",
		Toolset:     "memory",
		Description: "Search persisted memories via full-text search.",
		Emoji:       "🔍",
		Handler:     newMemorySearchHandler(store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "memory_search",
				Description: "Search previously saved memories. Empty query lists recent memories.",
				Parameters:  json.RawMessage(memorySearchSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "memory_delete",
		Toolset:     "memory",
		Description: "Delete a memory by ID.",
		Emoji:       "🗑",
		Handler:     newMemoryDeleteHandler(store),
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "memory_delete",
				Description: "Delete a memory by its ID.",
				Parameters:  json.RawMessage(memoryDeleteSchema),
			},
		},
	})
}
