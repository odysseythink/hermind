package obsidian

import (
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
)

func RegisterAll(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "obsidian_read_note",
		Toolset:     "obsidian",
		Description: "Read an Obsidian note, parsing front-matter and wikilinks.",
		Emoji:       "📓",
		Handler:     readNoteHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_read_note",
				Description: "Read an Obsidian note from the vault, parsing its front-matter and content.",
				Parameters:  json.RawMessage(readNoteSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_write_note",
		Toolset:     "obsidian",
		Description: "Write an Obsidian note with optional front-matter.",
		Emoji:       "✏️",
		Handler:     writeNoteHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_write_note",
				Description: "Write a new Obsidian note to the vault, optionally including front-matter.",
				Parameters:  json.RawMessage(writeNoteSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_search_vault",
		Toolset:     "obsidian",
		Description: "Search the Obsidian vault for notes matching a query.",
		Emoji:       "🔍",
		Handler:     searchVaultHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_search_vault",
				Description: "Search all notes in the Obsidian vault for a given text query.",
				Parameters:  json.RawMessage(searchVaultSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_list_links",
		Toolset:     "obsidian",
		Description: "List outgoing, incoming, or bidirectional wikilinks for a note.",
		Emoji:       "🔗",
		Handler:     listLinksHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_list_links",
				Description: "List wikilinks for a note, filtered by direction (outgoing, incoming, or both).",
				Parameters:  json.RawMessage(listLinksSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_update_front_matter",
		Toolset:     "obsidian",
		Description: "Update front-matter key-value pairs for an existing note.",
		Emoji:       "🏷️",
		Handler:     updateFrontMatterHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_update_front_matter",
				Description: "Merge key-value updates into an existing note's YAML front-matter.",
				Parameters:  json.RawMessage(updateFrontMatterSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_append_to_note",
		Toolset:     "obsidian",
		Description: "Append content to the end of an existing note.",
		Emoji:       "➕",
		Handler:     appendToNoteHandler,
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_append_to_note",
				Description: "Append raw content to the end of an existing Obsidian note.",
				Parameters:  json.RawMessage(appendToNoteSchema),
			},
		},
	})
}
