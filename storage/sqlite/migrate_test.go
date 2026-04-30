package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_FreshDBCreatesCurrentSchema(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	var ver string
	require.NoError(t, store.db.QueryRowContext(context.Background(),
		`SELECT value FROM schema_meta WHERE key='version'`).Scan(&ver))
	assert.Equal(t, "9", ver)
}

func TestMigrate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())
	require.NoError(t, store.Migrate())

	var ver string
	require.NoError(t, store.db.QueryRowContext(context.Background(),
		`SELECT value FROM schema_meta WHERE key='version'`).Scan(&ver))
	assert.Equal(t, "9", ver)
}

func TestMigrate_FreshDBHasNoSessionsTable(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	var name string
	err = store.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'`,
	).Scan(&name)
	require.Error(t, err, "expected no sessions table in v4 schema")
}

func TestMigrate_FreshDBSeedsConversationStateSingleton(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	var count int
	require.NoError(t, store.db.QueryRow(
		`SELECT COUNT(*) FROM conversation_state`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestMigrate_v6AddsReinforcementColumns(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	// All three new columns must accept values.
	_, err = store.db.Exec(`INSERT INTO memories (id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector, status, superseded_by, reinforcement_count, neglect_count, last_used_at)
        VALUES ('m1','','c','','[]','{}',0,0,'',NULL,'active','',3,1,123.4)`)
	require.NoError(t, err)

	var rc, nc int
	var lua float64
	require.NoError(t, store.db.QueryRow(
		`SELECT reinforcement_count, neglect_count, last_used_at FROM memories WHERE id = 'm1'`).Scan(&rc, &nc, &lua))
	assert.Equal(t, 3, rc)
	assert.Equal(t, 1, nc)
	assert.InDelta(t, 123.4, lua, 0.01)
}

func TestMigrate_v7AddsMemoryEventsTable(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	_, err = store.db.Exec(`INSERT INTO memory_events (ts, kind, data) VALUES (?, ?, ?)`,
		1712345678.0, "memory.consolidated", `{"scanned":10}`)
	require.NoError(t, err)

	var count int
	require.NoError(t, store.db.QueryRow(
		`SELECT COUNT(*) FROM memory_events WHERE kind = ?`, "memory.consolidated").Scan(&count))
	assert.Equal(t, 1, count)
}
