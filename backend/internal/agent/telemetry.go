package agent

import (
	"context"
	"reflect"
	"time"
)

func logChatStarted(eventLog eventLogger, userID *int, sessionUUID string, workspaceID int, provider, model string) {
	if isNilLogger(eventLog) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = eventLog.LogEvent(ctx, "agent_chat_started", map[string]any{
			"session_uuid": sessionUUID,
			"workspace_id": workspaceID,
			"provider":     provider,
			"model":        model,
		}, userID)
	}()
}

// isNilLogger handles the Go nil-interface trap: a typed nil pointer stored
// in an interface is not == nil.
func isNilLogger(el eventLogger) bool {
	if el == nil {
		return true
	}
	v := reflect.ValueOf(el)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

func logChatSent(eventLog eventLogger, userID *int, sessionUUID string, from, to string) {
	if isNilLogger(eventLog) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = eventLog.LogEvent(ctx, "agent_chat_sent", map[string]any{
			"session_uuid": sessionUUID,
			"from":         from,
			"to":           to,
		}, userID)
	}()
}

func logChatTerminated(eventLog eventLogger, userID *int, sessionUUID string, reason string, duration time.Duration) {
	if isNilLogger(eventLog) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = eventLog.LogEvent(ctx, "agent_chat_terminated", map[string]any{
			"session_uuid": sessionUUID,
			"reason":       reason,
			"duration_ms":  duration.Milliseconds(),
		}, userID)
	}()
}
