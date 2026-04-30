package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return d
}

// handleConversationGet responds to GET /api/conversation with the
// entire instance-scoped history.
func (s *Server) handleConversationGet(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 200)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)

	rows, err := s.opts.Storage.GetHistory(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]StoredMessageDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, StoredMessageDTO{
			ID:           row.ID,
			Role:         row.Role,
			Content:      row.Content,
			ToolCallID:   row.ToolCallID,
			ToolName:     row.ToolName,
			Timestamp:    float64(row.Timestamp.UnixNano()) / 1e9,
			FinishReason: row.FinishReason,
			Reasoning:    row.Reasoning,
		})
	}
	writeJSON(w, ConversationHistoryResponse{Messages: out})
}

// handleConversationPost accepts a user message, kicks off one
// engine turn, and returns 202. The engine streams its progress via
// the event hub (/api/sse).
func (s *Server) handleConversationPost(w http.ResponseWriter, r *http.Request) {
	if s.opts.Deps.Provider == nil {
		http.Error(w, "provider not configured", http.StatusServiceUnavailable)
		return
	}
	var body ConversationPostRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.UserMessage == "" {
		http.Error(w, "user_message required", http.StatusBadRequest)
		return
	}

	s.runMu.Lock()
	if s.runCancel != nil {
		s.runMu.Unlock()
		http.Error(w, "another turn is in flight", http.StatusConflict)
		return
	}
	runCtx, cancel := context.WithCancel(context.Background())
	s.runCancel = cancel
	s.runMu.Unlock()

	eng := agent.NewEngineWithToolsAndAux(
		s.opts.Deps.Provider, s.opts.Deps.AuxProvider, s.opts.Deps.Storage,
		s.opts.Deps.ToolReg, s.opts.Deps.AgentCfg, s.opts.Deps.Platform,
	)
	wireEngineToHub(eng, s.streams)

	go func() {
		defer func() {
			s.runMu.Lock()
			s.runCancel = nil
			s.runMu.Unlock()
			cancel()
		}()
		_, err := eng.RunConversation(runCtx, &agent.RunOptions{
			UserMessage: body.UserMessage,
			Model:       body.Model,
		})
		if err != nil {
			s.streams.Publish(StreamEvent{
				Type: EventTypeError,
				Data: map[string]any{"message": err.Error()},
			})
		}
		s.streams.Publish(StreamEvent{Type: EventTypeDone})
	}()

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, ConversationPostResponse{Accepted: true})
}

// handleConversationCancel cancels the in-flight turn, if any.
func (s *Server) handleConversationCancel(w http.ResponseWriter, _ *http.Request) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.runCancel != nil {
		s.runCancel()
		s.runCancel = nil
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationMessagePut(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	var req EditMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	if err := s.opts.Storage.UpdateMessage(r.Context(), id, req.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationMessageDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	if err := s.opts.Storage.DeleteMessage(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationMessageRegenerate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	// Delete this message and everything after it
	if err := s.opts.Storage.DeleteMessageAndAfter(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RegenerateResponse{Accepted: true})
}

// wireEngineToHub forwards engine callbacks to the stream hub so SSE
// subscribers see the turn progress in real time.
func wireEngineToHub(eng *agent.Engine, hub StreamHub) {
	eng.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
		if d == nil || d.Content == "" {
			return
		}
		hub.Publish(StreamEvent{
			Type: EventTypeMessageChunk,
			Data: map[string]any{"text": d.Content},
		})
	})
	eng.SetToolStartCallback(func(call message.ContentBlock) {
		hub.Publish(StreamEvent{
			Type: EventTypeToolCall,
			Data: map[string]any{
				"id":    call.ToolUseID,
				"name":  call.ToolUseName,
				"input": call.ToolUseInput,
			},
		})
	})
	eng.SetToolResultCallback(func(call message.ContentBlock, result string) {
		hub.Publish(StreamEvent{
			Type: EventTypeToolResult,
			Data: map[string]any{
				"id":     call.ToolUseID,
				"result": result,
			},
		})
	})
}
