package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestThreadCompaction_CRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&ThreadCompaction{}))

	// Create
	c := ThreadCompaction{
		WorkspaceID:   1,
		ThreadID:      intPtr(2),
		Summary:       "Summary of chats 1–5",
		UpToChatID:    5,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&c).Error)
	assert.NotZero(t, c.ID)

	// Read back
	var loaded ThreadCompaction
	require.NoError(t, db.First(&loaded, c.ID).Error)
	assert.Equal(t, 1, loaded.WorkspaceID)
	assert.Equal(t, 2, *loaded.ThreadID)
	assert.Equal(t, "Summary of chats 1–5", loaded.Summary)
	assert.Equal(t, 5, loaded.UpToChatID)

	// Nil ThreadID (default workspace session)
	c2 := ThreadCompaction{
		WorkspaceID:   1,
		ThreadID:      nil,
		Summary:       "Default session summary",
		UpToChatID:    10,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&c2).Error)
	var loaded2 ThreadCompaction
	require.NoError(t, db.First(&loaded2, c2.ID).Error)
	assert.Nil(t, loaded2.ThreadID)

	// Latest-for-query ordering
	c3 := ThreadCompaction{
		WorkspaceID:   1,
		ThreadID:      intPtr(2),
		Summary:       "Newer summary",
		UpToChatID:    8,
		CreatedAt:     time.Now().Add(time.Minute),
		LastUpdatedAt: time.Now().Add(time.Minute),
	}
	require.NoError(t, db.Create(&c3).Error)

	var latest ThreadCompaction
	require.NoError(t, db.Where("workspace_id = ? AND thread_id = ?", 1, 2).
		Order("created_at DESC").First(&latest).Error)
	assert.Equal(t, 8, latest.UpToChatID)
	assert.Equal(t, "Newer summary", latest.Summary)
}

func intPtr(i int) *int { return &i }
