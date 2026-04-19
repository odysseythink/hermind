package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleSessionMessages responds to GET /api/sessions/{id}/messages.
// Messages are returned in oldest-first order matching the underlying
// SQLite query. Pagination uses Limit + Offset query params.
func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)

	rows, err := s.opts.Storage.GetMessages(r.Context(), id, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	msgs := make([]MessageDTO, 0, len(rows))
	for _, row := range rows {
		dto := MessageDTO{
			ID:         row.ID,
			Role:       row.Role,
			Content:    row.Content,
			Timestamp:  toEpoch(row.Timestamp),
			TokenCount: row.TokenCount,
		}
		if len(row.ToolCalls) > 0 {
			dto.ToolCalls = string(row.ToolCalls)
		}
		msgs = append(msgs, dto)
	}
	writeJSON(w, MessagesResponse{Messages: msgs, Total: offset + len(msgs)})
}
