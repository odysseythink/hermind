package api

import (
	"net/http"
	"sort"
)

// handleToolsList responds to GET /api/tools with all registered tools
// and their current enabled/disabled status. The full registry is
// always exposed so the UI can list disabled tools for re-enabling.
func (s *Server) handleToolsList(w http.ResponseWriter, _ *http.Request) {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		writeJSON(w, ToolsResponse{Tools: []ToolDTO{}})
		return
	}

	disabled := s.disabledTools()
	entries := deps.ToolReg.Entries(nil)
	out := make([]ToolDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, ToolDTO{
			Name:        e.Name,
			Description: e.Description,
			Toolset:     e.Toolset,
			Enabled:     !disabled[e.Name],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, ToolsResponse{Tools: out})
}
