package api

import "net/http"

// handleMemoryStats returns aggregate memory counts for /api/memory/stats.
func (s *Server) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "storage not configured"})
		return
	}
	stats, err := s.opts.Storage.MemoryStats(r.Context())
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, stats)
}
