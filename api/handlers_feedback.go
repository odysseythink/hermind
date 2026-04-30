package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		MessageID int64 `json:"message_id"`
		Score     int   `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Score < -1 || req.Score > 1 {
		http.Error(w, "score must be -1, 0, or 1", http.StatusBadRequest)
		return
	}
	if err := s.opts.Storage.SaveFeedback(r.Context(), req.MessageID, req.Score); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
