// storage/sqlite/tx.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nousresearch/hermes-agent/storage"
)

// WithTx runs fn inside a single SQL transaction. The transaction is
// committed if fn returns nil, rolled back if fn returns an error or panics.
// Panics are re-raised after rollback.
func (s *Store) WithTx(ctx context.Context, fn func(tx storage.Tx) error) error {
	sqlTx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}

	txWrapper := &txImpl{store: s, tx: sqlTx}

	defer func() {
		if r := recover(); r != nil {
			_ = sqlTx.Rollback()
			panic(r) // re-raise after rollback
		}
	}()

	if err := fn(txWrapper); err != nil {
		if rbErr := sqlTx.Rollback(); rbErr != nil {
			return fmt.Errorf("sqlite: rollback after error %v: %w", err, rbErr)
		}
		return err
	}

	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

// txImpl implements storage.Tx by wrapping a *sql.Tx.
// It uses the same query logic as Store, but against the tx connection.
type txImpl struct {
	store *Store
	tx    *sql.Tx
}

// queryer is the common interface satisfied by both *sql.DB and *sql.Tx.
// This lets us share query logic between Store and txImpl.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (t *txImpl) CreateSession(ctx context.Context, session *storage.Session) error {
	modelConfig := string(session.ModelConfig)
	if modelConfig == "" {
		modelConfig = "{}"
	}
	_, err := t.tx.ExecContext(ctx, `
        INSERT INTO sessions (
            id, source, user_id, model, model_config, system_prompt,
            parent_session_id, started_at, message_count, tool_call_count,
            input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
            reasoning_tokens, title
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Source, session.UserID, session.Model,
		modelConfig, session.SystemPrompt, session.ParentSessionID,
		toEpoch(session.StartedAt), session.MessageCount, session.ToolCallCount,
		session.Usage.InputTokens, session.Usage.OutputTokens,
		session.Usage.CacheReadTokens, session.Usage.CacheWriteTokens,
		session.Usage.ReasoningTokens, session.Title,
	)
	if err != nil {
		return fmt.Errorf("sqlite tx: create session: %w", err)
	}
	return nil
}

func (t *txImpl) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	// Delegate to the store's non-tx logic but passing the tx connection
	// is tedious — for the minimal plan, reuse the query directly.
	var (
		sess        storage.Session
		modelConfig string
		startedAt   float64
		endedAt     sql.NullFloat64
	)
	err := t.tx.QueryRowContext(ctx, `
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
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite tx: get session: %w", err)
	}
	sess.ModelConfig = []byte(modelConfig)
	sess.StartedAt = fromEpoch(startedAt)
	if endedAt.Valid {
		ts := fromEpoch(endedAt.Float64)
		sess.EndedAt = &ts
	}
	return &sess, nil
}

func (t *txImpl) UpdateSession(ctx context.Context, id string, upd *storage.SessionUpdate) error {
	var setClauses []string
	var args []any
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
	if len(setClauses) == 0 {
		return nil
	}
	query := "UPDATE sessions SET " + joinWith(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("sqlite tx: update session: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (t *txImpl) AddMessage(ctx context.Context, sessionID string, msg *storage.StoredMessage) error {
	toolCalls := string(msg.ToolCalls)
	_, err := t.tx.ExecContext(ctx, `
        INSERT INTO messages (
            session_id, role, content, tool_call_id, tool_calls, tool_name,
            timestamp, token_count, finish_reason, reasoning, reasoning_details
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, msg.Role, msg.Content, msg.ToolCallID, toolCalls, msg.ToolName,
		toEpoch(msg.Timestamp), msg.TokenCount, msg.FinishReason,
		msg.Reasoning, msg.ReasoningDetails,
	)
	if err != nil {
		return fmt.Errorf("sqlite tx: add message: %w", err)
	}
	return nil
}

func (t *txImpl) UpdateSystemPrompt(ctx context.Context, sessionID, prompt string) error {
	res, err := t.tx.ExecContext(ctx,
		`UPDATE sessions SET system_prompt = ? WHERE id = ?`, prompt, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite tx: update system prompt: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (t *txImpl) UpdateUsage(ctx context.Context, sessionID string, usage *storage.UsageUpdate) error {
	res, err := t.tx.ExecContext(ctx, `
        UPDATE sessions SET
            input_tokens = input_tokens + ?,
            output_tokens = output_tokens + ?,
            cache_read_tokens = cache_read_tokens + ?,
            cache_write_tokens = cache_write_tokens + ?,
            reasoning_tokens = reasoning_tokens + ?,
            actual_cost_usd = actual_cost_usd + ?
        WHERE id = ?`,
		usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens,
		usage.CacheWriteTokens, usage.ReasoningTokens, usage.CostUSD, sessionID,
	)
	if err != nil {
		return fmt.Errorf("sqlite tx: update usage: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// Compile-time check that txImpl satisfies storage.Tx
var _ storage.Tx = (*txImpl)(nil)

// Silence unused-helper warnings if the queryer type isn't used inline
var _ queryer = (*sql.DB)(nil)
var _ queryer = (*sql.Tx)(nil)
