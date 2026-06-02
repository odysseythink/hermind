package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrate_IncludesThreadCompaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, AutoMigrate(db))

	// ThreadCompaction table should exist and accept writes
	c := models.ThreadCompaction{
		WorkspaceID: 1,
		Summary:     "test",
		UpToChatID:  1,
	}
	require.NoError(t, db.Create(&c).Error)
	assert.NotZero(t, c.ID)
}

func TestSeedDefaults_SetsCompressionEnabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}))
	require.NoError(t, SeedDefaults(db))

	var s models.SystemSetting
	require.NoError(t, db.Where("`key` = ?", "context_compress_enabled").First(&s).Error)
	require.NotNil(t, s.Value)
	assert.Equal(t, "false", *s.Value)
}
