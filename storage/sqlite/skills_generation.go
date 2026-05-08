// Package sqlite — skills_generation state-row accessors.
package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// GetSkillsGeneration implements storage.Storage.
func (s *Store) GetSkillsGeneration(ctx context.Context) (*storage.SkillsGeneration, error) {
	var (
		hash    string
		seq     int64
		updated float64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT hash, seq, updated_at FROM skills_generation WHERE id = 1`,
	).Scan(&hash, &seq, &updated)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get skills_generation: %w", err)
	}
	return &storage.SkillsGeneration{
		Hash:      hash,
		Seq:       seq,
		UpdatedAt: fromEpoch(updated),
	}, nil
}

// SetSkillsGeneration implements storage.Storage.
func (s *Store) SetSkillsGeneration(
	ctx context.Context,
	newHash string,
) (oldHash string, oldSeq, newSeq int64, bumped bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("sqlite: set skills_generation begin: %w", err)
	}
	defer tx.Rollback()

	var curHash string
	if err := tx.QueryRowContext(ctx,
		`SELECT hash, seq FROM skills_generation WHERE id = 1`,
	).Scan(&curHash, &oldSeq); err != nil {
		return "", 0, 0, false, fmt.Errorf("sqlite: set skills_generation read: %w", err)
	}

	if curHash == newHash {
		newSeq = oldSeq
		return curHash, oldSeq, newSeq, false, tx.Commit()
	}

	newSeq = oldSeq + 1
	if _, err := tx.ExecContext(ctx,
		`UPDATE skills_generation
		    SET hash = ?, seq = ?, updated_at = ?
		  WHERE id = 1`,
		newHash, newSeq, toEpoch(time.Now().UTC()),
	); err != nil {
		return "", 0, 0, false, fmt.Errorf("sqlite: set skills_generation update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", 0, 0, false, fmt.Errorf("sqlite: set skills_generation commit: %w", err)
	}
	return curHash, oldSeq, newSeq, true, nil
}
