// storage/sqlite/session.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// toEpoch converts a time.Time to a float64 Unix timestamp with sub-second precision.
// This matches the Python hermes state.db storage format.
func toEpoch(t time.Time) float64 {
	return float64(t.UnixNano()) / 1e9
}

// fromEpoch converts a float64 Unix timestamp back to time.Time.
func fromEpoch(f float64) time.Time {
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}

// CreateSession inserts a new session row.
func (s *Store) CreateSession(ctx context.Context, session *storage.Session) error {
	modelConfig := string(session.ModelConfig)
	if modelConfig == "" {
		modelConfig = "{}"
	}

	_, err := s.db.ExecContext(ctx, `
        INSERT INTO sessions (
            id, source, user_id, model, model_config, system_prompt,
            parent_session_id, started_at, message_count, tool_call_count,
            input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
            reasoning_tokens, title
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Source, session.UserID, session.Model,
		modelConfig, session.SystemPrompt,
		session.ParentSessionID, toEpoch(session.StartedAt),
		session.MessageCount, session.ToolCallCount,
		session.Usage.InputTokens, session.Usage.OutputTokens,
		session.Usage.CacheReadTokens, session.Usage.CacheWriteTokens,
		session.Usage.ReasoningTokens, session.Title,
	)
	if err != nil {
		return fmt.Errorf("sqlite: create session %s: %w", session.ID, err)
	}
	return nil
}

// GetSession fetches a session by ID. Returns storage.ErrNotFound if missing.
func (s *Store) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	var (
		sess        storage.Session
		modelConfig string
		startedAt   float64
		endedAt     sql.NullFloat64
	)
	err := s.db.QueryRowContext(ctx, `
        SELECT id, source, user_id, model, model_config, system_prompt,
               parent_session_id, started_at, ended_at, end_reason,
               message_count, tool_call_count,
               input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
               reasoning_tokens, billing_provider, billing_base_url,
               estimated_cost_usd, actual_cost_usd, cost_status, title
        FROM sessions WHERE id = ?`, id,
	).Scan(
		&sess.ID, &sess.Source, &sess.UserID, &sess.Model, &modelConfig,
		&sess.SystemPrompt, &sess.ParentSessionID, &startedAt, &endedAt,
		&sess.EndReason, &sess.MessageCount, &sess.ToolCallCount,
		&sess.Usage.InputTokens, &sess.Usage.OutputTokens,
		&sess.Usage.CacheReadTokens, &sess.Usage.CacheWriteTokens,
		&sess.Usage.ReasoningTokens, &sess.BillingProvider, &sess.BillingBaseURL,
		&sess.EstimatedCost, &sess.ActualCost, &sess.CostStatus, &sess.Title,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get session %s: %w", id, err)
	}

	sess.ModelConfig = []byte(modelConfig)
	sess.StartedAt = fromEpoch(startedAt)
	if endedAt.Valid {
		t := fromEpoch(endedAt.Float64)
		sess.EndedAt = &t
	}
	return &sess, nil
}

// UpdateSession applies partial updates to a session.
func (s *Store) UpdateSession(ctx context.Context, id string, upd *storage.SessionUpdate) error {
	// Build dynamic UPDATE based on non-nil fields
	var (
		setClauses []string
		args       []any
	)
	if upd.EndedAt != nil {
		setClauses = append(setClauses, "ended_at = ?")
		args = append(args, toEpoch(*upd.EndedAt))
	}
	if upd.EndReason != "" {
		setClauses = append(setClauses, "end_reason = ?")
		args = append(args, upd.EndReason)
	}
	if upd.Title != "" {
		setClauses = append(setClauses, "title = ?")
		args = append(args, upd.Title)
	}
	if upd.MessageCount != nil {
		setClauses = append(setClauses, "message_count = ?")
		args = append(args, *upd.MessageCount)
	}
	if upd.Model != nil {
		setClauses = append(setClauses, "model = ?")
		args = append(args, *upd.Model)
	}
	if upd.SystemPrompt != nil {
		setClauses = append(setClauses, "system_prompt = ?")
		args = append(args, *upd.SystemPrompt)
	}
	if len(setClauses) == 0 {
		return nil
	}

	query := "UPDATE sessions SET " + joinWith(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("sqlite: update session %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// joinWith is a tiny helper for building dynamic SQL.
func joinWith(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
