package sqlite

import (
	"context"
	"fmt"
)

func (s *Store) SaveFeedback(ctx context.Context, messageID int64, score int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feedback (message_id, score) VALUES (?, ?)
		 ON CONFLICT(message_id) DO UPDATE SET score = excluded.score`,
		messageID, score,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save feedback: %w", err)
	}
	return nil
}
