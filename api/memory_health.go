package api

import (
	"net/http"

	"github.com/odysseythink/hermind/storage"
)

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

	// Populate current skills generation (best-effort, errors are silent).
	if gen, err := s.opts.Storage.GetSkillsGeneration(r.Context()); err == nil && gen != nil {
		h.CurrentSkillsGeneration = &storage.CurrentSkillsGen{
			Hash:      gen.Hash,
			Seq:       gen.Seq,
			UpdatedAt: gen.UpdatedAt.Unix(),
		}
	}

	writeJSON(w, h)
}
