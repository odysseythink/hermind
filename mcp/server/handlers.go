package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// Server is the MCP server object.
type Server struct {
	opts *ServerOpts
}

// ServerOpts bundles dependencies.
type ServerOpts struct {
	Storage     storage.Storage
	Events      *EventBridge
	Permissions *PermissionQueue
}

// NewServer constructs a server from opts.
func NewServer(opts *ServerOpts) *Server { return &Server{opts: opts} }

// ---- initialize ----

func (s *Server) handleInitialize(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "hermind",
			"version": "dev",
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	})
}

// ---- tools/list ----

func (s *Server) handleToolsList(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"tools": BuiltinTools()})
}

// ---- tools/call ----

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p toolsCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	text, err := s.dispatchTool(ctx, p.Name, p.Arguments)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
	})
}

func (s *Server) dispatchTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "conversations_list":
		return s.conversationsList(ctx, args)
	case "conversation_get":
		return s.conversationGet(ctx, args)
	case "messages_read":
		return s.messagesRead(ctx, args)
	case "messages_send":
		return s.messagesSend(ctx, args)
	case "attachments_fetch":
		return s.attachmentsFetch(ctx, args)
	case "events_poll":
		return s.eventsPoll(ctx, args)
	case "events_wait":
		return s.eventsWait(ctx, args)
	case "permissions_list_open":
		return s.permissionsListOpen(ctx, args)
	case "permissions_respond":
		return s.permissionsRespond(ctx, args)
	case "channels_list":
		return s.channelsList(ctx, args)
	}
	return "", fmt.Errorf("mcp/server: unknown tool %q", name)
}

// ---- tool implementations ----

func (s *Server) conversationsList(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Platform string `json:"platform"`
		Limit    int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Limit <= 0 {
		a.Limit = 50
	}
	rows, err := s.opts.Storage.ListSessions(ctx, &storage.ListOptions{Limit: a.Limit})
	if err != nil {
		return "", err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		if a.Platform != "" && r.Source != a.Platform {
			continue
		}
		out = append(out, map[string]any{
			"session_key": r.ID,
			"session_id":  r.ID,
			"platform":    r.Source,
			"chat_name":   r.Title,
			"updated_at":  r.EndedAt,
		})
	}
	data, _ := json.MarshalIndent(map[string]any{
		"count":         len(out),
		"conversations": out,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) conversationGet(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		SessionKey string `json:"session_key"`
	}
	_ = json.Unmarshal(args, &a)
	if a.SessionKey == "" {
		return "", fmt.Errorf("session_key is required")
	}
	sess, err := s.opts.Storage.GetSession(ctx, a.SessionKey)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(sess, "", "  ")
	return string(data), nil
}

func (s *Server) messagesRead(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		SessionKey string `json:"session_key"`
		Limit      int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &a)
	if a.SessionKey == "" {
		return "", fmt.Errorf("session_key is required")
	}
	if a.Limit <= 0 {
		a.Limit = 50
	}
	msgs, err := s.opts.Storage.GetMessages(ctx, a.SessionKey, a.Limit, 0)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(map[string]any{
		"session_key": a.SessionKey,
		"count":       len(msgs),
		"messages":    msgs,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) messagesSend(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Target  string `json:"target"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Target == "" || a.Message == "" {
		return "", fmt.Errorf("target and message are required")
	}
	s.opts.Events.push(Event{
		Cursor:     time.Now().UnixNano(),
		Kind:       "message",
		SessionKey: a.Target,
		Role:       "outgoing",
		Content:    a.Message,
	})
	return `{"status":"queued"}`, nil
}

func (s *Server) attachmentsFetch(_ context.Context, _ json.RawMessage) (string, error) {
	// Placeholder — hermind doesn't expose attachments through
	// storage.Storage yet. Returning an empty list keeps hosts happy.
	return `{"count":0,"attachments":[]}`, nil
}

func (s *Server) eventsPoll(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		AfterCursor int64  `json:"after_cursor"`
		SessionKey  string `json:"session_key"`
		Limit       int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &a)
	events, next := s.opts.Events.Poll(a.AfterCursor, a.SessionKey, a.Limit)
	data, _ := json.MarshalIndent(map[string]any{
		"events":      events,
		"next_cursor": next,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) eventsWait(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		AfterCursor int64  `json:"after_cursor"`
		SessionKey  string `json:"session_key"`
		TimeoutMS   int    `json:"timeout_ms"`
	}
	_ = json.Unmarshal(args, &a)
	if a.TimeoutMS <= 0 {
		a.TimeoutMS = 30_000
	}
	ev, err := s.opts.Events.Wait(ctx, a.AfterCursor, a.SessionKey, time.Duration(a.TimeoutMS)*time.Millisecond)
	if err != nil {
		return "", err
	}
	if ev == nil {
		return `null`, nil
	}
	data, _ := json.Marshal(ev)
	return string(data), nil
}

func (s *Server) permissionsListOpen(_ context.Context, _ json.RawMessage) (string, error) {
	open := s.opts.Permissions.ListOpen()
	data, _ := json.MarshalIndent(map[string]any{
		"count":       len(open),
		"permissions": open,
	}, "", "  ")
	return string(data), nil
}

func (s *Server) permissionsRespond(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		ID       string `json:"id"`
		Decision string `json:"decision"`
	}
	_ = json.Unmarshal(args, &a)
	if !s.opts.Permissions.Respond(a.ID, a.Decision) {
		return "", fmt.Errorf("permission id %q not open", a.ID)
	}
	return `{"status":"recorded"}`, nil
}

func (s *Server) channelsList(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Platform string `json:"platform"`
	}
	_ = json.Unmarshal(args, &a)
	rows, err := s.opts.Storage.ListSessions(ctx, &storage.ListOptions{Limit: 200})
	if err != nil {
		return "", err
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, r := range rows {
		if a.Platform != "" && r.Source != a.Platform {
			continue
		}
		key := r.Source + ":" + r.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	data, _ := json.MarshalIndent(map[string]any{"channels": out}, "", "  ")
	return string(data), nil
}
