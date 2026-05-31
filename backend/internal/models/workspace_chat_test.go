package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitFTS5(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&WorkspaceChat{}))
	require.NoError(t, InitFTS5(db))

	// Insert into the FTS5 virtual table directly (manual sync).
	require.NoError(t, db.Exec(
		`INSERT INTO workspace_chat_fts(prompt, response) VALUES (?, ?)`,
		"hello world", "greetings earthling",
	).Error)

	// Query with MATCH.
	var count int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM workspace_chat_fts WHERE workspace_chat_fts MATCH ?`,
		"hello",
	).Scan(&count).Error)

	assert.Equal(t, int64(1), count)
}
