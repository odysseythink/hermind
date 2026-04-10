// storage/sqlite/message.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nousresearch/hermes-agent/storage"
)

// AddMessage inserts a new message row. The messages_fts virtual table
// is kept in sync automatically via AFTER INSERT trigger.
func (s *Store) AddMessage(ctx context.Context, sessionID string, msg *storage.StoredMessage) error {
	toolCalls := string(msg.ToolCalls)
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO messages (
            session_id, role, content, tool_call_id, tool_calls, tool_name,
            timestamp, token_count, finish_reason, reasoning, reasoning_details
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, msg.Role, msg.Content, msg.ToolCallID, toolCalls, msg.ToolName,
		toEpoch(msg.Timestamp), msg.TokenCount, msg.FinishReason,
		msg.Reasoning, msg.ReasoningDetails,
	)
	if err != nil {
		return fmt.Errorf("sqlite: add message to %s: %w", sessionID, err)
	}
	s.writeCount.Add(1)
	return nil
}

// GetMessages fetches messages for a session in timestamp order.
func (s *Store) GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]*storage.StoredMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, session_id, role, content, tool_call_id, tool_calls, tool_name,
               timestamp, token_count, finish_reason, reasoning, reasoning_details
        FROM messages WHERE session_id = ?
        ORDER BY timestamp ASC, id ASC
        LIMIT ? OFFSET ?`, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get messages for %s: %w", sessionID, err)
	}
	defer rows.Close()

	var msgs []*storage.StoredMessage
	for rows.Next() {
		var (
			m         storage.StoredMessage
			toolCalls string
			ts        float64
		)
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content,
			&m.ToolCallID, &toolCalls, &m.ToolName, &ts, &m.TokenCount,
			&m.FinishReason, &m.Reasoning, &m.ReasoningDetails); err != nil {
			return nil, fmt.Errorf("sqlite: scan message: %w", err)
		}
		if toolCalls != "" {
			m.ToolCalls = []byte(toolCalls)
		}
		m.Timestamp = fromEpoch(ts)
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

// SearchMessages performs FTS5 full-text search over message content.
func (s *Store) SearchMessages(ctx context.Context, query string, opts *storage.SearchOptions) ([]*storage.SearchResult, error) {
	limit := 50
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// MATCH query against the FTS5 virtual table
	sqlQuery := `
        SELECT m.id, m.session_id, m.role, m.content, m.timestamp,
               snippet(messages_fts, 0, '<mark>', '</mark>', '...', 16) AS snippet,
               rank
        FROM messages_fts
        JOIN messages m ON m.id = messages_fts.rowid
        WHERE messages_fts MATCH ?
    `
	args := []any{query}
	if opts != nil && opts.SessionID != "" {
		sqlQuery += " AND m.session_id = ?"
		args = append(args, opts.SessionID)
	}
	sqlQuery += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: search: %w", err)
	}
	defer rows.Close()

	var results []*storage.SearchResult
	for rows.Next() {
		var (
			m       storage.StoredMessage
			ts      float64
			snippet string
			rank    float64
		)
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content,
			&ts, &snippet, &rank); err != nil {
			return nil, fmt.Errorf("sqlite: scan search result: %w", err)
		}
		m.Timestamp = fromEpoch(ts)
		results = append(results, &storage.SearchResult{
			Message:   &m,
			SessionID: m.SessionID,
			Snippet:   snippet,
			Rank:      rank,
		})
	}
	return results, rows.Err()
}

// ListSessions returns sessions ordered by started_at DESC.
func (s *Store) ListSessions(ctx context.Context, opts *storage.ListOptions) ([]*storage.Session, error) {
	limit := 50
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	query := `SELECT id, source, user_id, model, started_at, title
              FROM sessions`
	var (
		args       []any
		whereAdded bool
	)
	if opts != nil {
		if opts.Source != "" {
			query += " WHERE source = ?"
			args = append(args, opts.Source)
			whereAdded = true
		}
		if opts.UserID != "" {
			if whereAdded {
				query += " AND user_id = ?"
			} else {
				query += " WHERE user_id = ?"
				whereAdded = true
			}
			args = append(args, opts.UserID)
		}
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		var (
			sess      storage.Session
			startedAt float64
		)
		if err := rows.Scan(&sess.ID, &sess.Source, &sess.UserID,
			&sess.Model, &startedAt, &sess.Title); err != nil {
			return nil, fmt.Errorf("sqlite: scan session: %w", err)
		}
		sess.StartedAt = fromEpoch(startedAt)
		sessions = append(sessions, &sess)
	}
	return sessions, rows.Err()
}

// UpdateSystemPrompt caches the prompt for Anthropic prefix-caching reuse.
func (s *Store) UpdateSystemPrompt(ctx context.Context, sessionID, prompt string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET system_prompt = ? WHERE id = ?`, prompt, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite: update system prompt: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// UpdateUsage adds to the running usage counters for a session.
func (s *Store) UpdateUsage(ctx context.Context, sessionID string, usage *storage.UsageUpdate) error {
	_, err := s.db.ExecContext(ctx, `
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
		return fmt.Errorf("sqlite: update usage: %w", err)
	}
	return nil
}

// Helper to silence unused imports if needed
var _ = sql.ErrNoRows
