package api

import "net/http"

// handleToolsList responds to GET /api/tools. It returns an empty list
// by default: the REPL-wired tool registry is per-session and not
// trivially reachable from the read-only web surface without pulling
// the agent/ package into our dependency graph. The WebSocket agent
// can replace this by mounting its own route later, or we can extend
// ServerOpts with a tool registry when the frontend needs it.
func (s *Server) handleToolsList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, ToolsResponse{Tools: []ToolDTO{}})
}
