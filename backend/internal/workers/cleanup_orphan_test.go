package workers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCleanupOrphanJob(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create table manually to avoid gorm:"default:now()" SQLite syntax issue
	err = db.Exec(`CREATE TABLE workspace_parsed_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT UNIQUE,
		workspace_id INTEGER,
		user_id INTEGER,
		thread_id INTEGER,
		metadata TEXT,
		token_count_estimate INTEGER DEFAULT 0,
		created_at DATETIME
	)`).Error
	require.NoError(t, err)

	tmpDir := t.TempDir()
	cfg := &config.Config{StorageDir: tmpDir}

	// Create orphan file
	err = os.WriteFile(filepath.Join(tmpDir, "orphan.txt"), []byte("x"), 0644)
	require.NoError(t, err)

	// Create referenced file
	err = os.WriteFile(filepath.Join(tmpDir, "referenced.txt"), []byte("y"), 0644)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.WorkspaceParsedFile{Filename: "referenced.txt"}).Error)

	job := NewCleanupOrphanJob(db, cfg)
	err = job.Run(context.Background())
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "orphan.txt"))
	assert.True(t, os.IsNotExist(err), "orphan file should be deleted")

	_, err = os.Stat(filepath.Join(tmpDir, "referenced.txt"))
	assert.NoError(t, err, "referenced file should remain")
}
