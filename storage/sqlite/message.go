// storage/sqlite/message.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// AppendMessage inserts a new message row. The messages_fts virtual
// table is kept in sync automatically via AFTER INSERT trigger.
func (s *Store) AppendMessage(ctx context.Context, msg *storage.StoredMessage) error {
	return insertMessageExec(ctx, s.db, msg)
}

func (t *txImpl) AppendMessage(ctx context.Context, msg *storage.StoredMessage) error {
	return insertMessageExec(ctx, t.tx, msg)
}

func insertMessageExec(ctx context.Context, ex execer, msg *storage.StoredMessage) error {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now().UTC()
	}
	res, err := ex.ExecContext(ctx, `
		INSERT INTO messages
		  (role, content, tool_call_id, tool_calls, tool_name,
		   timestamp, token_count, finish_reason, reasoning, reasoning_details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.Role, msg.Content, msg.ToolCallID,
		string(msg.ToolCalls), msg.ToolName,
		toEpoch(msg.Timestamp),
		msg.TokenCount, msg.FinishReason, msg.Reasoning, msg.ReasoningDetails,
	)
	if err != nil {
		return fmt.Errorf("sqlite: append message: %w", err)
	}
	msg.ID, _ = res.LastInsertId()
	return nil
}

// GetHistory returns messages in insertion order (id ASC).
func (s *Store) GetHistory(ctx context.Context, limit, offset int) ([]*storage.StoredMessage, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, role, content, tool_call_id, tool_calls, tool_name,
		       timestamp, token_count, finish_reason, reasoning, reasoning_details
		FROM messages
		ORDER BY id ASC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get history: %w", err)
	}
	defer rows.Close()

	var out []*storage.StoredMessage
	for rows.Next() {
		m := &storage.StoredMessage{}
		var ts float64
		var toolCalls string
		if err := rows.Scan(
			&m.ID, &m.Role, &m.Content, &m.ToolCallID,
			&toolCalls, &m.ToolName, &ts, &m.TokenCount,
			&m.FinishReason, &m.Reasoning, &m.ReasoningDetails,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan message: %w", err)
		}
		m.Timestamp = fromEpoch(ts)
		if toolCalls != "" {
			m.ToolCalls = []byte(toolCalls)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SearchMessages performs FTS5 full-text search.
func (s *Store) SearchMessages(
	ctx context.Context,
	query string,
	opts *storage.SearchOptions,
) ([]*storage.SearchResult, error) {
	limit := 20
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.role, m.content, m.timestamp,
		       snippet(messages_fts, 0, '<mark>', '</mark>', '...', 16) AS snip,
		       rank
		FROM messages m
		JOIN messages_fts ON m.id = messages_fts.rowid
		WHERE messages_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: search messages: %w", err)
	}
	defer rows.Close()

	var out []*storage.SearchResult
	for rows.Next() {
		sm := &storage.StoredMessage{}
		var ts float64
		var rank float64
		var snippet string
		if err := rows.Scan(&sm.ID, &sm.Role, &sm.Content, &ts, &snippet, &rank); err != nil {
			return nil, fmt.Errorf("sqlite: scan search: %w", err)
		}
		sm.Timestamp = fromEpoch(ts)
		out = append(out, &storage.SearchResult{Message: sm, Snippet: snippet, Rank: rank})
	}
	return out, rows.Err()
}

func (s *Store) UpdateMessage(ctx context.Context, id int64, content string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET content = ? WHERE id = ?`, content, id)
	return err
}

func (s *Store) DeleteMessage(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteMessageAndAfter(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE id >= ?`, id)
	return err
}

func (s *Store) DeleteMessagesAfter(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE id > ?`, id)
	return err
}
