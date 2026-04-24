package api

import "net/http"

// handleMemoryHealth returns schema + FTS health for /api/memory/health.
func (s *Server) handleMemoryHealth(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "storage not configured"})
		return
	}
	h, err := s.opts.Storage.MemoryHealth(r.Context())
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, h)
}
