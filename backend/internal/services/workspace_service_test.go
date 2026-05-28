package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWSDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

func TestWorkspaceService_UpdatePin(t *testing.T) {
	db := setupWSDB(t)
	svc := NewWorkspaceService(db, &config.Config{})

	ws := &models.Workspace{Name: "ws1", Slug: "ws1"}
	require.NoError(t, db.Create(ws).Error)
	f := false
	doc := &models.WorkspaceDocument{
		DocId:       "doc-1",
		Filename:    "a.txt",
		Docpath:     "custom-documents/a.txt-xyz.json",
		WorkspaceID: ws.ID,
		Pinned:      &f,
	}
	require.NoError(t, db.Create(doc).Error)

	// Pin
	err := svc.UpdatePin(context.Background(), ws.ID, "custom-documents/a.txt-xyz.json", true)
	require.NoError(t, err)

	var got models.WorkspaceDocument
	require.NoError(t, db.First(&got, doc.ID).Error)
	require.NotNil(t, got.Pinned)
	assert.True(t, *got.Pinned)

	// Unpin
	err = svc.UpdatePin(context.Background(), ws.ID, "custom-documents/a.txt-xyz.json", false)
	require.NoError(t, err)
	require.NoError(t, db.First(&got, doc.ID).Error)
	assert.False(t, *got.Pinned)
}

func TestWorkspaceService_UpdatePin_NotFound(t *testing.T) {
	db := setupWSDB(t)
	svc := NewWorkspaceService(db, &config.Config{})

	err := svc.UpdatePin(context.Background(), 9999, "missing.json", true)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
