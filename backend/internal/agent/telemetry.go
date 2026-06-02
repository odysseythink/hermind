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

func logCompactionFinished(eventLog eventLogger, userID *int, workspaceID int, path string, beforeTokens, afterTokens int, fallbackUsed bool) {
	if isNilLogger(eventLog) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		savedPct := 0.0
		if beforeTokens > 0 {
			savedPct = float64(beforeTokens-afterTokens) / float64(beforeTokens) * 100
		}
		_ = eventLog.LogEvent(ctx, "compaction_finished", map[string]any{
			"workspace_id":  workspaceID,
			"path":          path,
			"before_tokens": beforeTokens,
			"after_tokens":  afterTokens,
			"saved_pct":     savedPct,
			"fallback_used": fallbackUsed,
		}, userID)
	}()
}
