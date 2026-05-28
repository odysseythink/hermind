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

func TestCleanupGeneratedJob(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceChat{}))

	tmpDir := t.TempDir()
	outputsDir := filepath.Join(tmpDir, "outputs")
	require.NoError(t, os.MkdirAll(outputsDir, 0755))
	cfg := &config.Config{StorageDir: tmpDir}

	// Create unreferenced generated file (matches pattern)
	err = os.WriteFile(filepath.Join(outputsDir, "file-550e8400-e29b-41d4-a716-446655440000.txt"), []byte("x"), 0644)
	require.NoError(t, err)

	// Create referenced generated file
	err = os.WriteFile(filepath.Join(outputsDir, "file-550e8400-e29b-41d4-a716-446655440001.txt"), []byte("y"), 0644)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.WorkspaceChat{
		Response: "file-550e8400-e29b-41d4-a716-446655440001.txt",
		Include:  true,
	}).Error)

	job := NewCleanupGeneratedJob(db, cfg)
	err = job.Run(context.Background())
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputsDir, "file-550e8400-e29b-41d4-a716-446655440000.txt"))
	assert.True(t, os.IsNotExist(err), "unreferenced file should be deleted")

	_, err = os.Stat(filepath.Join(outputsDir, "file-550e8400-e29b-41d4-a716-446655440001.txt"))
	assert.NoError(t, err, "referenced file should remain")
}
