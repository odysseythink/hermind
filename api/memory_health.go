package api

import (
	"net/http"
	"time"

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

	// Populate presence (best-effort, errors are silent).
	if p := s.opts.Deps.Presence; p != nil {
		now := time.Now()
		srcs := p.Sources(now)
		view := &storage.PresenceView{
			Available: p.Available(now),
			Sources:   make([]storage.PresenceSourceView, 0, len(srcs)),
		}
		for _, sv := range srcs {
			view.Sources = append(view.Sources, storage.PresenceSourceView{
				Name: sv.Name,
				Vote: sv.Vote.String(),
			})
		}
		h.Presence = view
	}

	writeJSON(w, h)
}
