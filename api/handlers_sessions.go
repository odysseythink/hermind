package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	id := sessionIDParam(r)
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

// handleSessionPatch applies partial updates to a session. All fields are
// optional; a missing JSON key means "leave unchanged", an explicit empty
// string clears the field (where applicable).
func (s *Server) handleSessionPatch(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := sessionIDParam(r)
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}
	var body struct {
		Title        *string `json:"title,omitempty"`
		SystemPrompt *string `json:"system_prompt,omitempty"`
		Model        *string `json:"model,omitempty"`
	}
	// Cap total body at MaxSystemPromptBytes + 1KB JSON overhead so a
	// malicious client cannot force the server to buffer an unbounded
	// request before the per-field length checks below run.
	r.Body = http.MaxBytesReader(w, r.Body, MaxSystemPromptBytes+1024)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	upd := &storage.SessionUpdate{}

	if body.Title != nil {
		title := strings.TrimSpace(*body.Title)
		if title == "" {
			http.Error(w, "title must not be empty", http.StatusBadRequest)
			return
		}
		if len(title) > MaxSessionTitleBytes {
			http.Error(w, "title too long", http.StatusBadRequest)
			return
		}
		upd.Title = title
	}
	if body.SystemPrompt != nil {
		if len(*body.SystemPrompt) > MaxSystemPromptBytes {
			http.Error(w, "system_prompt too long", http.StatusBadRequest)
			return
		}
		upd.SystemPrompt = body.SystemPrompt
	}
	if body.Model != nil {
		if len(*body.Model) > MaxModelNameBytes {
			http.Error(w, "model name too long", http.StatusBadRequest)
			return
		}
		upd.Model = body.Model
	}

	err := s.opts.Storage.UpdateSession(r.Context(), id, upd)
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sess, err := s.opts.Storage.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Only broadcast if at least one field was actually changed. An empty
	// body `{}` hits UpdateSession's no-op fast path, and we should not
	// fan out a session_updated event that carries no new information.
	if s.streams != nil && (body.Title != nil || body.SystemPrompt != nil || body.Model != nil) {
		s.streams.Publish(StreamEvent{
			Type:      EventTypeSessionUpdated,
			SessionID: id,
			Data: map[string]any{
				"title":         sess.Title,
				"model":         sess.Model,
				"system_prompt": sess.SystemPrompt,
			},
		})
	}
	writeJSON(w, dtoFromSession(sess))
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
		SystemPrompt: s.SystemPrompt,
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
