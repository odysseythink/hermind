package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkspace_CompressFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&Workspace{}))

	// Create with compression fields
	ws := Workspace{
		Name:               "test-ws",
		Slug:               "test-ws",
		CompressEnabled:    boolPtr(true),
		CompressThreshold:  floatPtr(0.65),
		CompressContextLen: intPtr(16384),
	}
	require.NoError(t, db.Create(&ws).Error)
	assert.NotZero(t, ws.ID)

	// Read back
	var loaded Workspace
	require.NoError(t, db.First(&loaded, ws.ID).Error)
	assert.NotNil(t, loaded.CompressEnabled)
	assert.True(t, *loaded.CompressEnabled)
	assert.NotNil(t, loaded.CompressThreshold)
	assert.InDelta(t, 0.65, *loaded.CompressThreshold, 0.001)
	assert.NotNil(t, loaded.CompressContextLen)
	assert.Equal(t, 16384, *loaded.CompressContextLen)

	// Nil fields (use global default)
	ws2 := Workspace{Name: "test-ws-2", Slug: "test-ws-2"}
	require.NoError(t, db.Create(&ws2).Error)
	var loaded2 Workspace
	require.NoError(t, db.First(&loaded2, ws2.ID).Error)
	assert.Nil(t, loaded2.CompressEnabled)
	assert.Nil(t, loaded2.CompressThreshold)
	assert.Nil(t, loaded2.CompressContextLen)
}

func TestWorkspaceThread_ParentThreadID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.AutoMigrate(&WorkspaceThread{}))

	// Thread with parent
	wt := WorkspaceThread{
		Name:           "child",
		Slug:           "child",
		WorkspaceID:    1,
		ParentThreadID: intPtr(99),
	}
	require.NoError(t, db.Create(&wt).Error)

	var loaded WorkspaceThread
	require.NoError(t, db.First(&loaded, wt.ID).Error)
	assert.NotNil(t, loaded.ParentThreadID)
	assert.Equal(t, 99, *loaded.ParentThreadID)

	// Thread without parent
	wt2 := WorkspaceThread{Name: "orphan", Slug: "orphan", WorkspaceID: 1}
	require.NoError(t, db.Create(&wt2).Error)
	var loaded2 WorkspaceThread
	require.NoError(t, db.First(&loaded2, wt2.ID).Error)
	assert.Nil(t, loaded2.ParentThreadID)
}

func boolPtr(b bool) *bool         { return &b }
func floatPtr(f float64) *float64 { return &f }
