package api

import (
	"net/http"
	"strconv"
	"strings"
)

// handleMemoryReport returns a paginated stream of recent memory events
// for /api/memory/report. Supported query params: limit, offset, kind
// (comma-separated kinds).
func (s *Server) handleMemoryReport(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "storage not configured"})
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	var kinds []string
	if v := r.URL.Query().Get("kind"); v != "" {
		kinds = strings.Split(v, ",")
	}
	events, err := s.opts.Storage.ListMemoryEvents(r.Context(), limit, offset, kinds)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"events": events})
}
