package api

import (
	"net/http"
)

// Placeholder handlers — full implementations land in Task C2.

func (s *Server) handleConversationGet(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "conversation handlers not wired yet", http.StatusServiceUnavailable)
}

func (s *Server) handleConversationPost(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "conversation handlers not wired yet", http.StatusServiceUnavailable)
}

func (s *Server) handleConversationCancel(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
