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

func TestContinueInThread_CreatesWorkspaceAndThread(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ScheduledJob{}, &models.ScheduledJobRun{}, &models.Workspace{}, &models.WorkspaceThread{}, &models.WorkspaceChat{}, &models.WorkspaceUser{}))

	sjSvc := NewScheduledJobService(db)
	job, _ := sjSvc.Create(context.Background(), ScheduledJobInput{
		Name: "c", Prompt: "do X", Schedule: "* * * * *",
	})
	run, _ := sjSvc.StartRun(context.Background(), job.ID)
	_ = sjSvc.Complete(context.Background(), run.ID, `{"text":"the answer is 42"}`)

	contSvc := NewScheduledJobContinueService(db, sjSvc)
	ws, thr, err := contSvc.ContinueInThread(context.Background(), run.ID)
	require.NoError(t, err)
	require.NotNil(t, ws)
	require.NotNil(t, thr)
	assert.Equal(t, "scheduled-jobs", ws.Slug)

	var chats []models.WorkspaceChat
	require.NoError(t, db.Where("workspace_id = ? AND thread_id = ?", ws.ID, thr.ID).Find(&chats).Error)
	require.Len(t, chats, 1)
	assert.Equal(t, "do X", chats[0].Prompt)
}
