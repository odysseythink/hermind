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

func newMemTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Memory{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func TestMemoryService_CreateAndList(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 10
	_, err := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "fact A")
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "fact B")
	require.NoError(t, err)

	rows, err := svc.ListWorkspace(context.Background(), &uid, wid)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "fact B", rows[0].Content) // DESC by createdAt
}

func TestMemoryService_GlobalLimit(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid := 7
	for i := 0; i < models.GlobalMemoryLimit; i++ {
		_, err := svc.Create(context.Background(), &uid, nil, models.MemoryScopeGlobal, "g")
		require.NoError(t, err)
	}
	_, err := svc.Create(context.Background(), &uid, nil, models.MemoryScopeGlobal, "g")
	assert.ErrorIs(t, err, ErrMemoryLimitReached)
}

func TestMemoryService_PromoteAndDemote(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 3, 5
	m, _ := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "x")

	promoted, err := svc.PromoteToGlobal(context.Background(), m.ID)
	require.NoError(t, err)
	assert.Equal(t, models.MemoryScopeGlobal, promoted.Scope)
	assert.Nil(t, promoted.WorkspaceID)

	demoted, err := svc.DemoteToWorkspace(context.Background(), m.ID, wid)
	require.NoError(t, err)
	assert.Equal(t, models.MemoryScopeWorkspace, demoted.Scope)
	require.NotNil(t, demoted.WorkspaceID)
	assert.Equal(t, wid, *demoted.WorkspaceID)
}

func TestMemoryService_ApplyExtracted(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 1
	existing, _ := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "old")

	res, err := svc.ApplyExtracted(context.Background(), &uid, wid, []ExtractedAction{
		{Action: "create", Scope: "WORKSPACE", Content: "new ws"},
		{Action: "create", Scope: "GLOBAL", Content: "new global"},
		{Action: "update", UpdateID: &existing.ID, Content: "old revised"},
	}, models.GlobalMemoryLimit)
	require.NoError(t, err)
	assert.Equal(t, 1, res.WS)
	assert.Equal(t, 1, res.Global)
	assert.Equal(t, 1, res.Updated)

	rows, _ := svc.ListWorkspace(context.Background(), &uid, wid)
	var found bool
	for _, r := range rows {
		if r.ID == existing.ID && r.Content == "old revised" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestMemoryService_ReplaceWorkspace_Transactional(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 1
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "x")
	}
	require.NoError(t, svc.ReplaceWorkspace(context.Background(), &uid, wid, []string{"a", "b"}))
	rows, _ := svc.ListWorkspace(context.Background(), &uid, wid)
	assert.Len(t, rows, 2)
}

func TestMemoryService_CreateEmptyContent(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 1
	_, err := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "  ")
	assert.ErrorContains(t, err, "content cannot be empty")
}

func TestMemoryService_UpdateEmptyContent(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 1
	m, _ := svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "x")
	_, err := svc.Update(context.Background(), m.ID, "   ")
	assert.ErrorContains(t, err, "content cannot be empty")
}

func TestMemoryService_GetNotFound(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	_, err := svc.Get(context.Background(), 9999)
	assert.ErrorIs(t, err, ErrMemoryNotFound)
}

func TestMemoryService_ApplyExtractedRespectsLimits(t *testing.T) {
	svc := NewMemoryService(newMemTestDB(t))
	uid, wid := 1, 1
	// Fill workspace to limit
	for i := 0; i < models.WorkspaceMemoryLimit; i++ {
		_, _ = svc.Create(context.Background(), &uid, &wid, models.MemoryScopeWorkspace, "x")
	}
	res, err := svc.ApplyExtracted(context.Background(), &uid, wid, []ExtractedAction{
		{Action: "create", Scope: "WORKSPACE", Content: "overflow"},
	}, models.GlobalMemoryLimit)
	require.NoError(t, err)
	assert.Equal(t, 0, res.WS) // capped by actual DB count inside tx
}
