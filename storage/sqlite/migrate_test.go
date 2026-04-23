package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_FreshDBCreatesV3Schema(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	var ver string
	require.NoError(t, store.db.QueryRowContext(context.Background(),
		`SELECT value FROM schema_meta WHERE key='version'`).Scan(&ver))
	assert.Equal(t, "3", ver)
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
	assert.Equal(t, "3", ver)
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
	require.Error(t, err, "expected no sessions table in v3 schema")
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
