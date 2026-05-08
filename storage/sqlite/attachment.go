package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

func (s *Store) SaveAttachment(ctx context.Context, msgID int64, name string, mimeType string, url string, size int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO attachments (message_id, name, type, url, size) VALUES (?, ?, ?, ?, ?)`,
		msgID, name, mimeType, url, size,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save attachment: %w", err)
	}
	return nil
}

func (s *Store) ListAttachments(ctx context.Context, msgID int64) ([]storage.Attachment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, message_id, name, type, url, size, created_at FROM attachments WHERE message_id = ? ORDER BY id ASC`,
		msgID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list attachments: %w", err)
	}
	defer rows.Close()

	var out []storage.Attachment
	for rows.Next() {
		var a storage.Attachment
		var createdAtStr string
		if err := rows.Scan(&a.ID, &a.MessageID, &a.Name, &a.Type, &a.URL, &a.Size, &createdAtStr); err != nil {
			return nil, fmt.Errorf("sqlite: scan attachment: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
			a.CreatedAt = t.UTC()
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
