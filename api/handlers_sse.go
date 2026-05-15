package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/odysseythink/mlog"
)

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	mlog.Info("SSE connection started")
	flusher, ok := w.(http.Flusher)
	if !ok {
		mlog.Error("SSE streaming not supported")
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	mlog.Info("SSE headers set, flushing...")

	events, unsub := s.streams.Subscribe()
	defer unsub()

	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(map[string]any{
				"type": ev.Type,
				"data": ev.Data,
			})
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
