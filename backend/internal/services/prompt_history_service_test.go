package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPHTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PromptHistory{}))
	return db
}

func TestPromptHistoryService_LogAndList(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)
	uid := 7

	require.NoError(t, svc.Log(context.Background(), 100, "old prompt one", &uid))
	require.NoError(t, svc.Log(context.Background(), 100, "old prompt two", &uid))
	require.NoError(t, svc.Log(context.Background(), 200, "different workspace", nil))

	rows, err := svc.ListByWorkspace(context.Background(), 100, 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// DESC order — newest first
	assert.Equal(t, "old prompt two", rows[0].Prompt)
	assert.Equal(t, "old prompt one", rows[1].Prompt)
	assert.Equal(t, 7, *rows[0].ModifiedBy)
}

func TestPromptHistoryService_Delete(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)

	require.NoError(t, svc.Log(context.Background(), 1, "p1", nil))
	require.NoError(t, svc.Log(context.Background(), 1, "p2", nil))

	rows, _ := svc.ListByWorkspace(context.Background(), 1, 10)
	require.Len(t, rows, 2)
	require.NoError(t, svc.Delete(context.Background(), rows[0].ID))
	rows, _ = svc.ListByWorkspace(context.Background(), 1, 10)
	assert.Len(t, rows, 1)
}

func TestPromptHistoryService_DeleteAll(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)
	require.NoError(t, svc.Log(context.Background(), 1, "p1", nil))
	require.NoError(t, svc.Log(context.Background(), 1, "p2", nil))
	require.NoError(t, svc.Log(context.Background(), 2, "p3", nil))

	require.NoError(t, svc.DeleteAll(context.Background(), 1))
	rows, _ := svc.ListByWorkspace(context.Background(), 1, 10)
	assert.Len(t, rows, 0)
	rows, _ = svc.ListByWorkspace(context.Background(), 2, 10)
	assert.Len(t, rows, 1)
}

func TestPromptHistoryService_ListLimit(t *testing.T) {
	db := newPHTestDB(t)
	svc := NewPromptHistoryService(db)
	for i := 0; i < 5; i++ {
		require.NoError(t, svc.Log(context.Background(), 1, "p", nil))
	}
	rows, err := svc.ListByWorkspace(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}
