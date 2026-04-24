package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// AppendMemoryEvent implements storage.Storage.
func (s *Store) AppendMemoryEvent(ctx context.Context, ts time.Time, kind string, data []byte) error {
	if len(data) == 0 {
		data = []byte("{}")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memory_events (ts, kind, data) VALUES (?, ?, ?)`,
		toEpoch(ts), kind, string(data))
	if err != nil {
		return fmt.Errorf("sqlite: append memory_event: %w", err)
	}
	return nil
}

// ListMemoryEvents implements storage.Storage.
func (s *Store) ListMemoryEvents(ctx context.Context, limit, offset int, kinds []string) ([]*storage.MemoryEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	var (
		rows *sql.Rows
		err  error
	)
	if len(kinds) == 0 {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, ts, kind, data FROM memory_events ORDER BY ts DESC LIMIT ? OFFSET ?`,
			limit, offset)
	} else {
		placeholders := strings.Repeat("?,", len(kinds))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, 0, len(kinds)+2)
		for _, k := range kinds {
			args = append(args, k)
		}
		args = append(args, limit, offset)
		q := fmt.Sprintf(
			`SELECT id, ts, kind, data FROM memory_events WHERE kind IN (%s) ORDER BY ts DESC LIMIT ? OFFSET ?`,
			placeholders)
		rows, err = s.db.QueryContext(ctx, q, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: list memory_events: %w", err)
	}
	defer rows.Close()
	out := make([]*storage.MemoryEvent, 0, limit)
	for rows.Next() {
		var (
			id   int64
			ts   float64
			kind string
			data string
		)
		if err := rows.Scan(&id, &ts, &kind, &data); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory_event: %w", err)
		}
		out = append(out, &storage.MemoryEvent{
			ID: id, TS: fromEpoch(ts), Kind: kind, Data: []byte(data),
		})
	}
	return out, rows.Err()
}
