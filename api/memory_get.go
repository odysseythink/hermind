package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/storage"
)

// handleMemoryGet returns a single memory for /api/memory/{id}. Returns
// 404 when not found. The raw vector bytes are stripped from the response
// to keep JSON compact.
func (s *Server) handleMemoryGet(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "storage not configured"})
		return
	}
	id := chi.URLParam(r, "id")
	mem, err := s.opts.Storage.GetMemory(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := *mem
	out.Vector = nil
	writeJSON(w, out)
}
