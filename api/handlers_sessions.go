package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/storage"
)

func (s *Server) handleSessionsList(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 20)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)

	// The Storage.ListOptions does not expose an offset field. Fetch
	// limit+offset rows and trim client-side. Fine for UI-scale
	// listings; replace with a proper paginated query if we ever care
	// about 100k+ sessions.
	fetch := limit + offset
	if fetch <= 0 {
		fetch = limit
	}
	rows, err := s.opts.Storage.ListSessions(r.Context(), &storage.ListOptions{Limit: fetch})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if offset > len(rows) {
		offset = len(rows)
	}
	rows = rows[offset:]
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]SessionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, dtoFromSession(row))
	}
	total := offset + len(rows)
	if len(rows) == limit {
		total++ // hint "more" without lying about an exact count
	}
	writeJSON(w, SessionListResponse{Sessions: out, Total: total})
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	row, err := s.opts.Storage.GetSession(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, dtoFromSession(row))
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, _ *http.Request) {
	// The minimal Storage interface does not include DeleteSession. The
	// MVP surfaces 501 so the frontend knows to hide the delete button
	// until Plan X extends the interface.
	http.Error(w, "session deletion not supported in MVP", http.StatusNotImplemented)
}

func dtoFromSession(s *storage.Session) SessionDTO {
	var endedAt float64
	if s.EndedAt != nil {
		endedAt = toEpoch(*s.EndedAt)
	}
	return SessionDTO{
		ID:           s.ID,
		Source:       s.Source,
		Model:        s.Model,
		StartedAt:    toEpoch(s.StartedAt),
		EndedAt:      endedAt,
		MessageCount: s.MessageCount,
		Title:        s.Title,
	}
}

// toEpoch mirrors storage/sqlite.toEpoch so DTOs expose Unix-seconds
// timestamps without importing the sqlite package.
func toEpoch(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	return float64(t.UnixNano()) / 1e9
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}
