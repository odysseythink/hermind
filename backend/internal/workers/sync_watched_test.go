package workers

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSyncWatchedJob_Enabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.Exec(`CREATE TABLE document_sync_queues (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		stale_after_ms INTEGER DEFAULT 604800000,
		next_sync_at DATETIME,
		created_at DATETIME,
		last_synced_at DATETIME,
		workspace_doc_id INTEGER UNIQUE
	)`).Error
	require.NoError(t, err)

	cfg := &config.Config{WorkerSyncWatchedEnabled: true}
	job := NewSyncWatchedJob(db, cfg, nil)

	assert.False(t, job.Enabled(context.Background()))

	require.NoError(t, db.Exec(
		"INSERT INTO document_sync_queues (next_sync_at, workspace_doc_id) VALUES (?, ?)",
		time.Now().Add(-1*time.Hour), 1,
	).Error)
	assert.True(t, job.Enabled(context.Background()))
}

func TestSyncWatchedJob_Run(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceDocument{}))

	err = db.Exec(`CREATE TABLE document_sync_queues (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		stale_after_ms INTEGER DEFAULT 604800000,
		next_sync_at DATETIME,
		created_at DATETIME,
		last_synced_at DATETIME,
		workspace_doc_id INTEGER UNIQUE
	)`).Error
	require.NoError(t, err)

	cfg := &config.Config{WorkerSyncWatchedEnabled: true}
	job := NewSyncWatchedJob(db, cfg, nil)

	// No stale queues
	err = job.Run(context.Background())
	require.NoError(t, err)

	// Add document and stale queue
	doc := models.WorkspaceDocument{DocId: "doc-1", Filename: "test.txt"}
	require.NoError(t, db.Create(&doc).Error)

	require.NoError(t, db.Exec(
		"INSERT INTO document_sync_queues (next_sync_at, workspace_doc_id, stale_after_ms) VALUES (?, ?, ?)",
		time.Now().Add(-1*time.Hour), doc.ID, 86400000,
	).Error)

	err = job.Run(context.Background())
	require.NoError(t, err)

	// Verify queue was updated
	var nextSync time.Time
	require.NoError(t, db.Raw("SELECT next_sync_at FROM document_sync_queues WHERE id = 1").Scan(&nextSync).Error)
	assert.True(t, nextSync.After(time.Now()), "next_sync_at should be in the future")
}
