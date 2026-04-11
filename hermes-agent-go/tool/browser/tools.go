package browser

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nousresearch/hermes-agent/tool"
)

const createSchema = `{"type":"object","properties":{}}`

const closeSchema = `{
  "type":"object",
  "properties":{"session_id":{"type":"string"}},
  "required":["session_id"]
}`

const liveSchema = `{
  "type":"object",
  "properties":{"session_id":{"type":"string"}},
  "required":["session_id"]
}`

func newCreateHandler(p Provider, store *SessionStore) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		sess, err := p.CreateSession(ctx)
		if err != nil {
			return tool.ToolError(err.Error()), nil
		}
		store.Put(sess)
		return tool.ToolResult(sess), nil
	}
}

func newCloseHandler(p Provider, store *SessionStore) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if strings.TrimSpace(args.SessionID) == "" {
			return tool.ToolError("session_id is required"), nil
		}
		if err := p.CloseSession(ctx, args.SessionID); err != nil {
			return tool.ToolError(err.Error()), nil
		}
		store.Delete(args.SessionID)
		return tool.ToolResult(map[string]any{"ok": true, "session_id": args.SessionID}), nil
	}
}

func newLiveURLHandler(p Provider, store *SessionStore) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if strings.TrimSpace(args.SessionID) == "" {
			return tool.ToolError("session_id is required"), nil
		}
		url, err := p.LiveURL(ctx, args.SessionID)
		if err != nil {
			return tool.ToolError(err.Error()), nil
		}
		if sess, ok := store.Get(args.SessionID); ok {
			sess.LiveURL = url
		}
		return tool.ToolResult(map[string]any{"session_id": args.SessionID, "live_url": url}), nil
	}
}
