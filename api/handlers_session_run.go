package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/api/sessionrun"
)

func (s *Server) handleSessionMessagesPost(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeErr(w, http.StatusBadRequest, "missing session id")
		return
	}
	var req MessageSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Text == "" {
		writeErr(w, http.StatusBadRequest, "text is required")
		return
	}
	// Provider being nil means BuildEngineDeps was never attached — the
	// server is meta-only. Return 503 so the UI can surface "no provider".
	if s.deps.Provider == nil {
		writeErr(w, http.StatusServiceUnavailable,
			"provider not configured; open Config panel to set api_key")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	if !s.registry.Register(sessionID, cancel) {
		cancel()
		writeErr(w, http.StatusConflict, "session busy")
		return
	}

	go func() {
		defer s.registry.Clear(sessionID)
		_ = sessionrun.Run(ctx, s.deps, sessionrun.Request{
			SessionID:   sessionID,
			UserMessage: req.Text,
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(MessageSubmitResponse{SessionID: sessionID, Status: "accepted"})
}

func (s *Server) handleSessionCancel(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeErr(w, http.StatusBadRequest, "missing session id")
		return
	}
	if !s.registry.Cancel(sessionID) {
		writeErr(w, http.StatusNotFound, "session not running")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
