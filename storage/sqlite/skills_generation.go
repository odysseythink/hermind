// Package sqlite — skills_generation state-row accessors.
package sqlite

import (
	"context"
	"fmt"

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
