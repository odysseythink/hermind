package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMigrateV8AddsSkillsGenerationTableAndColumn(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	// skills_generation table exists with single seeded row.
	var hash string
	var seq int64
	err = store.db.QueryRow(`SELECT hash, seq FROM skills_generation WHERE id = 1`).Scan(&hash, &seq)
	require.NoError(t, err)
	require.Equal(t, "", hash)
	require.Equal(t, int64(0), seq)

	// memories.reinforced_at_seq column exists, default 0.
	_, err = store.db.Exec(`INSERT INTO memories (id, content, created_at, updated_at) VALUES ('m1', 'x', 0, 0)`)
	require.NoError(t, err)
	var reinforcedSeq int64
	err = store.db.QueryRow(`SELECT reinforced_at_seq FROM memories WHERE id = 'm1'`).Scan(&reinforcedSeq)
	require.NoError(t, err)
	require.Equal(t, int64(0), reinforcedSeq)
}

func TestMigrateV8Idempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())
	require.NoError(t, store.Migrate()) // second run must not fail
	_ = context.Background()
}

func TestGetSkillsGenerationFreshDB(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()
	gen, err := store.GetSkillsGeneration(context.Background())
	require.NoError(t, err)
	require.Equal(t, "", gen.Hash)
	require.Equal(t, int64(0), gen.Seq)
	require.True(t, gen.UpdatedAt.IsZero() || gen.UpdatedAt.Unix() <= time.Now().Unix())
}
