package api

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

// handleSessionStreamWS serves GET /api/sessions/{id}/stream/ws. It
// upgrades the HTTP connection to WebSocket, subscribes to the
// StreamHub for that session, and forwards every event as a JSON text
// frame until the client disconnects or the session stops producing.
//
// The endpoint is read-only from the client's perspective: the server
// ignores inbound frames other than control frames. A bidirectional
// chat runner (client → server prompt submission) is a follow-up; the
// REST POST /api/sessions/{id}/messages is how prompts are pushed
// today.
func (s *Server) handleSessionStreamWS(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The web UI is bound to 127.0.0.1 by default; accept only
		// localhost origins so a rogue page on the open web can't
		// piggyback on the user's token.
		OriginPatterns: []string{"localhost", "localhost:*", "127.0.0.1", "127.0.0.1:*"},
	})
	if err != nil {
		// Accept already wrote the response code/body.
		return
	}
	// Best-effort close; normal completion overrides this via a later
	// explicit Close(StatusNormalClosure, "").
	defer conn.Close(websocket.StatusInternalError, "stream closed")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Drain inbound frames so ping/close frames are processed. We
	// discard payloads — this endpoint is read-only for the client.
	go func() {
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				cancel()
				return
			}
		}
	}()

	ch, id := s.streams.Subscribe(ctx, sessionID)
	defer s.streams.Unsubscribe(sessionID, id)

	// Keepalive ping so intermediate proxies / idle-killers don't
	// sever the connection during a long quiet stretch.
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				conn.Close(websocket.StatusNormalClosure, "stream ended")
				return
			}
			data, err := encodeStreamEvent(ev)
			if err != nil {
				continue
			}
			writeCtx, cancelWrite := context.WithTimeout(ctx, 5*time.Second)
			werr := conn.Write(writeCtx, websocket.MessageText, data)
			cancelWrite()
			if werr != nil {
				return
			}
		case <-heartbeat.C:
			pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
			perr := conn.Ping(pingCtx)
			cancelPing()
			if perr != nil {
				return
			}
		}
	}
}
