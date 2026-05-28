package workers

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newExtractMemDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceChat{}, &models.Memory{}, &models.Workspace{}, &models.User{}, &models.SystemSetting{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func TestExtractMemoriesJob_NoWork(t *testing.T) {
	db := newExtractMemDB(t)
	sysSvc := services.NewSystemService(db)
	memSvc := services.NewMemoryService(db)
	job := NewExtractMemoriesJob(db, memSvc, nil, sysSvc)
	require.NoError(t, job.Run(context.Background()))
}

func TestExtractMemoriesJob_SkipsSmallGroups(t *testing.T) {
	db := newExtractMemDB(t)
	sysSvc := services.NewSystemService(db)
	memSvc := services.NewMemoryService(db)
	uid, wid := 1, 1
	for i := 0; i < MinChatsForExtract-1; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: wid, UserID: &uid, Prompt: "q", Response: "a", Include: true,
			CreatedAt: time.Now().Add(-time.Hour),
		}).Error)
	}
	job := NewExtractMemoriesJob(db, memSvc, nil, sysSvc)
	require.NoError(t, job.Run(context.Background()))

	var processed int64
	db.Model(&models.WorkspaceChat{}).Where("memory_processed = ?", true).Count(&processed)
	assert.Equal(t, int64(0), processed)
}

func TestExtractMemoriesJob_SkipsActiveGroups(t *testing.T) {
	db := newExtractMemDB(t)
	sysSvc := services.NewSystemService(db)
	memSvc := services.NewMemoryService(db)
	uid, wid := 1, 1
	for i := 0; i < MinChatsForExtract; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: wid, UserID: &uid, Prompt: "q", Response: "a", Include: true,
			CreatedAt: time.Now().Add(-time.Minute), // only 1 min ago — still active
		}).Error)
	}
	job := NewExtractMemoriesJob(db, memSvc, nil, sysSvc)
	require.NoError(t, job.Run(context.Background()))

	var processed int64
	db.Model(&models.WorkspaceChat{}).Where("memory_processed = ?", true).Count(&processed)
	assert.Equal(t, int64(0), processed)
}

func TestExtractMemoriesJob_MarkProcessedDirectly(t *testing.T) {
	db := newExtractMemDB(t)
	chat := models.WorkspaceChat{WorkspaceID: 1, Prompt: "q", Response: "a", Include: true}
	require.NoError(t, db.Create(&chat).Error)

	job := NewExtractMemoriesJob(db, nil, nil, nil)
	job.markProcessed(context.Background(), []int{chat.ID})

	var processed int64
	db.Model(&models.WorkspaceChat{}).Where("memory_processed = ?", true).Count(&processed)
	assert.Equal(t, int64(1), processed)
}

func TestExtractMemoriesJob_FindsRecords(t *testing.T) {
	db := newExtractMemDB(t)
	uid, wid := 1, 1
	for i := 0; i < MinChatsForExtract; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: wid, UserID: &uid, Prompt: "q", Response: "a", Include: true,
			CreatedAt: time.Now().Add(-time.Hour),
		}).Error)
	}

	var unprocessed []models.WorkspaceChat
	err := db.Where("(memory_processed IS NULL OR memory_processed = ?) AND include = ?", false, true).
		Order("created_at ASC").Limit(1000).Find(&unprocessed).Error
	require.NoError(t, err)
	assert.Len(t, unprocessed, MinChatsForExtract)
}

func TestExtractMemoriesJob_MarksProcessedOnSuccess(t *testing.T) {
	db := newExtractMemDB(t)
	sysSvc := services.NewSystemService(db)
	memSvc := services.NewMemoryService(db)
	uid, wid := 1, 1
	for i := 0; i < MinChatsForExtract; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{
			WorkspaceID: wid, UserID: &uid, Prompt: "q", Response: "a", Include: true,
			CreatedAt: time.Now().Add(-time.Hour),
		}).Error)
	}
	// stub extractor that does nothing but succeeds
	stubExt := services.NewMemoryExtractor(memSvc, &stubExtractLLM{}, "", "")
	job := NewExtractMemoriesJob(db, memSvc, stubExt, sysSvc)
	err := job.Run(context.Background())
	require.NoError(t, err)

	var processed int64
	db.Model(&models.WorkspaceChat{}).Where("memory_processed = ?", true).Count(&processed)
	assert.Equal(t, int64(MinChatsForExtract), processed)
}

type stubExtractLLM struct{}

func (s *stubExtractLLM) Generate(_ context.Context, _ *core.Request) (*core.Response, error) {
	return &core.Response{}, nil
}
