// Package acp implements a minimal but complete Agent Communication
// Protocol server. It exposes sessions, event streaming, tool
// listing/execution, a .well-known/agent.json registry, and a
// simple token-based permission model.
//
// This is the full implementation referenced as Phase 17 in the
// gap closure roadmap. The older gateway/platforms/acp.go adapter
// remains in place as a backwards-compatible single-endpoint stub.
package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/tool"
)

// Server is the full ACP HTTP surface. It wraps a MessageHandler
// (provided by the Gateway) and a tool.Registry so the model can be
// called from ACP clients.
type Server struct {
	Addr      string
	Name      string // human-friendly agent name for the registry
	IconURL   string
	tools     *tool.Registry
	handler   gateway.MessageHandler
	perms     *Permissions
	events    *EventBus
	sessionMu sync.Mutex
	sessions  map[string]*Session
}

// NewServer builds a new ACP server.
func NewServer(addr, name string, toolReg *tool.Registry, handler gateway.MessageHandler, perms *Permissions) *Server {
	if addr == "" {
		addr = ":8083"
	}
	if name == "" {
		name = "hermes"
	}
	return &Server{
		Addr:     addr,
		Name:     name,
		tools:    toolReg,
		handler:  handler,
		perms:    perms,
		events:   NewEventBus(),
		sessions: map[string]*Session{},
	}
}

// Session is the ACP session state owned by the server.
type Session struct {
	ID        string
	CreatedAt time.Time
	User      string
}

// Name implements the gateway.Platform interface so the ACP server
// can be registered alongside other adapters if desired.
func (s *Server) PlatformName() string { return "acp_full" }

// Run starts the HTTP server and blocks until ctx is done.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent.json", s.handleWellKnown)
	mux.HandleFunc("/acp/sessions", s.handleCreateSession)
	mux.HandleFunc("/acp/messages", s.handleMessage)
	mux.HandleFunc("/acp/events", s.handleEvents)
	mux.HandleFunc("/acp/tools", s.handleListTools)
	mux.HandleFunc("/acp/tools/execute", s.handleExecuteTool)

	srv := &http.Server{Addr: s.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("acp: %w", err)
	}
}

func (s *Server) checkPermission(r *http.Request, action string) error {
	if s.perms == nil {
		return nil
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		return errors.New("missing bearer token")
	}
	if !s.perms.Allow(token, action) {
		return fmt.Errorf("permission denied: %s", action)
	}
	return nil
}

func (s *Server) handleWellKnown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name":        s.Name,
		"description": "Hermes Agent — Go port",
		"protocol":    "acp",
		"version":     "1.0",
		"icon":        s.IconURL,
		"endpoints": map[string]string{
			"sessions":      "/acp/sessions",
			"messages":      "/acp/messages",
			"events":        "/acp/events",
			"tools":         "/acp/tools",
			"tools_execute": "/acp/tools/execute",
		},
	})
}

type createSessionRequest struct {
	User string `json:"user"`
}

type createSessionResponse struct {
	SessionID string `json:"session_id"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := s.checkPermission(r, "sessions:create"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	var req createSessionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.User == "" {
		req.User = "anonymous"
	}
	id := fmt.Sprintf("acp-%d", time.Now().UnixNano())
	s.sessionMu.Lock()
	s.sessions[id] = &Session{ID: id, CreatedAt: time.Now().UTC(), User: req.User}
	s.sessionMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(createSessionResponse{SessionID: id})
}

type messageRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

type messageResponse struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := s.checkPermission(r, "messages:send"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	var req messageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	s.sessionMu.Lock()
	sess, ok := s.sessions[req.SessionID]
	s.sessionMu.Unlock()
	if !ok {
		http.Error(w, "unknown session", 404)
		return
	}
	in := gateway.IncomingMessage{
		Platform: "acp_full",
		UserID:   sess.User,
		ChatID:   sess.ID,
		Text:     req.Text,
	}
	// Publish a "message_received" event for subscribers.
	s.events.Publish(Event{Type: "message_received", SessionID: sess.ID, Data: req.Text})
	out, err := s.handler(r.Context(), in)
	if err != nil {
		s.events.Publish(Event{Type: "error", SessionID: sess.ID, Data: err.Error()})
		http.Error(w, err.Error(), 500)
		return
	}
	reply := ""
	if out != nil {
		reply = out.Text
	}
	s.events.Publish(Event{Type: "message_reply", SessionID: sess.ID, Data: reply})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(messageResponse{SessionID: sess.ID, Text: reply})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.checkPermission(r, "events:subscribe"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := s.events.Subscribe()
	defer s.events.Unsubscribe(ch)
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			buf, _ := json.Marshal(ev)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(buf))
			flusher.Flush()
		}
	}
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if err := s.checkPermission(r, "tools:list"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if s.tools == nil {
		_, _ = io.WriteString(w, "[]")
		return
	}
	defs := s.tools.Definitions(nil)
	out := make([]map[string]any, 0, len(defs))
	for _, d := range defs {
		out = append(out, map[string]any{
			"name":        d.Function.Name,
			"description": d.Function.Description,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type execRequest struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

func (s *Server) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := s.checkPermission(r, "tools:execute"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if s.tools == nil {
		http.Error(w, "no tool registry", 400)
		return
	}
	var req execRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	res, err := s.tools.Dispatch(r.Context(), req.Name, req.Args)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"result":`)
	_, _ = io.WriteString(w, res)
	_, _ = io.WriteString(w, `}`)
}
