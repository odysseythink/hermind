package api

import (
	"net/http"
	"path/filepath"
)

// handleSkillsStats returns skills directory aggregates for /api/skills/stats.
func (s *Server) handleSkillsStats(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "storage not configured"})
		return
	}
	skillsDir := filepath.Join(s.opts.InstanceRoot, "skills")
	stats, err := s.opts.Storage.SkillsStats(r.Context(), skillsDir)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, stats)
}
