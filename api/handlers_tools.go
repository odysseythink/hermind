package api

import (
	"net/http"
	"sort"

	"github.com/odysseythink/hermind/tool"
)

// handleToolsList responds to GET /api/tools with all registered tools
// and their current enabled/disabled status. The full registry is
// always exposed so the UI can list disabled tools for re-enabling.
// File toolset entries are aggregated into a single "filesystem" entry.
func (s *Server) handleToolsList(w http.ResponseWriter, _ *http.Request) {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		writeJSON(w, ToolsResponse{Tools: []ToolDTO{}})
		return
	}

	disabled := s.disabledTools()
	entries := deps.ToolReg.Entries(nil)

	// Collect file toolset entries for aggregation
	var fileTools []*tool.Entry
	var otherTools []*tool.Entry
	for _, e := range entries {
		if e.Toolset == "file" {
			fileTools = append(fileTools, e)
		} else {
			otherTools = append(otherTools, e)
		}
	}

	out := make([]ToolDTO, 0, len(otherTools)+1)

	// Add non-file tools (skip the virtual filesystem entry — it gets the aggregated DTO below)
	for _, e := range otherTools {
		if e.Name == "filesystem" {
			continue
		}
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
