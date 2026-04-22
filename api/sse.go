package api

import (
	"fmt"
	"net/http"
	"time"
)

// handleSessionStreamSSE serves GET /api/sessions/{id}/stream/sse. It
// implements the Server-Sent Events protocol: one
// `data: <json>\n\n` frame per StreamHub event, plus a comment-only
// heartbeat every 30s to keep proxies from idling the connection out.
//
// Useful for observability dashboards / curl-style debugging that
// don't want a bidirectional channel; the frame shape matches the
// WebSocket endpoint exactly so a frontend can pick either transport
// without duplicating parsing logic.
func (s *Server) handleSessionStreamSSE(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionIDParam(r)
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	ch, id := s.streams.Subscribe(ctx, sessionID)
	defer s.streams.Unsubscribe(sessionID, id)

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := encodeStreamEvent(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			// SSE comment lines (starting with ":") are ignored by
			// parsers but count as traffic for idle proxies.
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
