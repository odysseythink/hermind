package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

func (s *Store) GetProfile(ctx context.Context, userID string) (*storage.Profile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT version, updated_at FROM profiles WHERE user_id = ?`, userID)
	var (
		version int64
		updated float64
	)
	if err := row.Scan(&version, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("sqlite: get profile %s: %w", userID, err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kind, key, value, evidence, source_turns, confidence, updated_at
		 FROM profile_sections WHERE user_id = ? ORDER BY kind, key`, userID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list profile sections: %w", err)
	}
	defer rows.Close()

	p := &storage.Profile{UserID: userID, Version: version, UpdatedAt: fromEpoch(updated)}
	for rows.Next() {
		var (
			sec            storage.ProfileSection
			srcJSON        string
			sectionUpdated float64
		)
		if err := rows.Scan(&sec.ID, &sec.Kind, &sec.Key, &sec.Value,
			&sec.Evidence, &srcJSON, &sec.Confidence, &sectionUpdated); err != nil {
			return nil, fmt.Errorf("sqlite: scan profile section: %w", err)
		}
		sec.UserID = userID
		sec.UpdatedAt = fromEpoch(sectionUpdated)
		_ = json.Unmarshal([]byte(srcJSON), &sec.SourceTurns)
		p.Sections = append(p.Sections, sec)
	}
	return p, rows.Err()
}

func (s *Store) SaveProfileDelta(ctx context.Context, d *storage.ProfileDelta) (int64, error) {
	if d == nil || d.UserID == "" {
		return 0, fmt.Errorf("sqlite: SaveProfileDelta requires UserID")
	}
	var newVersion int64
	err := s.WithTx(ctx, func(tx storage.Tx) error {
		sqlTx := tx.(*txImpl).tx
		now := toEpoch(time.Now().UTC())

		// Upsert profile row + bump version.
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO profiles (user_id, version, updated_at)
			VALUES (?, 1, ?)
			ON CONFLICT(user_id) DO UPDATE SET
				version = version + 1,
				updated_at = excluded.updated_at`,
			d.UserID, now); err != nil {
			return fmt.Errorf("upsert profile: %w", err)
		}
		if err := sqlTx.QueryRowContext(ctx,
			`SELECT version FROM profiles WHERE user_id = ?`, d.UserID).
			Scan(&newVersion); err != nil {
			return fmt.Errorf("read version: %w", err)
		}

		for _, del := range d.Deletes {
			if _, err := sqlTx.ExecContext(ctx,
				`DELETE FROM profile_sections WHERE user_id = ? AND kind = ? AND key = ?`,
				del.UserID, del.Kind, del.Key); err != nil {
				return fmt.Errorf("delete %s/%s: %w", del.Kind, del.Key, err)
			}
		}
		for _, sec := range append(d.Adds, d.Updates...) {
			srcJSON, _ := json.Marshal(sec.SourceTurns)
			if _, err := sqlTx.ExecContext(ctx, `
				INSERT INTO profile_sections (user_id, kind, key, value, evidence, source_turns, confidence, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(user_id, kind, key) DO UPDATE SET
					value = excluded.value,
					evidence = excluded.evidence,
					source_turns = excluded.source_turns,
					confidence = excluded.confidence,
					updated_at = excluded.updated_at`,
				d.UserID, sec.Kind, sec.Key, sec.Value, sec.Evidence,
				string(srcJSON), sec.Confidence, now); err != nil {
				return fmt.Errorf("upsert %s/%s: %w", sec.Kind, sec.Key, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return newVersion, nil
}
