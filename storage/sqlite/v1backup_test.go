package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func makeV1SchemaFile(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY); CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT);`)
	require.NoError(t, err)
}

func TestBackupLegacyDBIfNeeded_RenamesWhenSessionsTableExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	makeV1SchemaFile(t, path)

	backedUp, backupPath, err := backupLegacyDBIfNeeded(path)
	require.NoError(t, err)
	assert.True(t, backedUp)
	assert.Equal(t, filepath.Join(dir, "state.db.v1-backup"), backupPath)

	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)
}

func TestBackupLegacyDBIfNeeded_NoOpWhenNoSessionsTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE messages (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	db.Close()

	backedUp, _, err := backupLegacyDBIfNeeded(path)
	require.NoError(t, err)
	assert.False(t, backedUp)

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestBackupLegacyDBIfNeeded_NoOpWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	backedUp, _, err := backupLegacyDBIfNeeded(filepath.Join(dir, "nope.db"))
	require.NoError(t, err)
	assert.False(t, backedUp)
}

func TestOpen_BacksUpLegacyV1DB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	makeV1SchemaFile(t, path)

	store, err := Open(path)
	require.NoError(t, err)
	defer store.Close()

	// Backup file exists next to fresh empty DB.
	_, err = os.Stat(filepath.Join(dir, "state.db.v1-backup"))
	require.NoError(t, err)
}

func TestBackupLegacyDBIfNeeded_AppendsSuffixOnCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	makeV1SchemaFile(t, path)

	collide := filepath.Join(dir, "state.db.v1-backup")
	require.NoError(t, os.WriteFile(collide, []byte("prior"), 0o644))

	backedUp, backupPath, err := backupLegacyDBIfNeeded(path)
	require.NoError(t, err)
	assert.True(t, backedUp)
	assert.True(t,
		strings.HasPrefix(filepath.Base(backupPath), "state.db.v1-backup."),
		"got %s, want prefix state.db.v1-backup.", backupPath)

	for _, p := range []string{collide, backupPath} {
		_, err := os.Stat(p)
		assert.NoError(t, err, "expected %s to exist", p)
	}
}
