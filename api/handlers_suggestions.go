package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"suggestions": []string{
			"What can you help me with?",
			"Explain this codebase",
			"Write a test for the current function",
			"Summarize the recent changes",
		},
	})
}
