package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupHandoffDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.WorkspaceThread{}, &models.ThreadCompaction{}, &models.WorkspaceChat{}))
	return db
}

func TestThreadService_Create_WithParentThreadID(t *testing.T) {
	db := setupHandoffDB(t)
	svc := NewThreadService(db)
	compStore := agentcompression.NewCompactionStore(db)

	// Create parent workspace + thread
	ws := &models.Workspace{Name: "Parent WS", Slug: "parent-ws"}
	require.NoError(t, db.Create(ws).Error)

	parentThread := &models.WorkspaceThread{Name: "Parent", Slug: "parent", WorkspaceID: ws.ID}
	require.NoError(t, db.Create(parentThread).Error)

	// Seed parent thread with a compaction
	require.NoError(t, compStore.Save(&models.ThreadCompaction{
		WorkspaceID: ws.ID,
		ThreadID:    &parentThread.ID,
		Summary:     "Parent summary",
		UpToChatID:  5,
	}))

	// Create child thread with ParentThreadID
	childReq := dto.CreateThreadRequest{
		Name:           "Child",
		Slug:           "child",
		ParentThreadID: &parentThread.ID,
	}
	child, err := svc.Create(context.Background(), ws.ID, nil, childReq)
	require.NoError(t, err)
	require.NotNil(t, child)
	assert.Equal(t, parentThread.ID, *child.ParentThreadID)

	// Child should inherit parent's latest compaction as seed
	seed, err := compStore.LoadLatest(ws.ID, &child.ID)
	require.NoError(t, err)
	require.NotNil(t, seed)
	assert.Equal(t, "Parent summary", seed.Summary)
	assert.Equal(t, 0, seed.UpToChatID)
}

func TestThreadService_Create_WithoutParentThreadID(t *testing.T) {
	db := setupHandoffDB(t)
	svc := NewThreadService(db)

	ws := &models.Workspace{Name: "Root", Slug: "root"}
	require.NoError(t, db.Create(ws).Error)

	rootReq := dto.CreateThreadRequest{Name: "Root Thread", Slug: "root-thread"}
	root, err := svc.Create(context.Background(), ws.ID, nil, rootReq)
	require.NoError(t, err)
	require.Nil(t, root.ParentThreadID)
}
