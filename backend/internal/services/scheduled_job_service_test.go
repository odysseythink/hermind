package services

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSJTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ScheduledJob{}, &models.ScheduledJobRun{}))
	return db
}

func TestScheduledJobService_Create_ValidCron(t *testing.T) {
	svc := NewScheduledJobService(newSJTestDB(t))
	job, err := svc.Create(context.Background(), ScheduledJobInput{
		Name: "weekly summary", Prompt: "summarize this week", Schedule: "0 9 * * 1",
	})
	require.NoError(t, err)
	assert.NotZero(t, job.ID)
	require.NotNil(t, job.NextRunAt)
	assert.True(t, job.NextRunAt.After(time.Now().Add(-time.Minute)))
}

func TestScheduledJobService_Create_InvalidCron(t *testing.T) {
	svc := NewScheduledJobService(newSJTestDB(t))
	_, err := svc.Create(context.Background(), ScheduledJobInput{
		Name: "bad", Prompt: "p", Schedule: "not-a-cron",
	})
	assert.ErrorIs(t, err, ErrInvalidCron)
}

func TestScheduledJobService_StartRun_Dedup(t *testing.T) {
	svc := NewScheduledJobService(newSJTestDB(t))
	job, _ := svc.Create(context.Background(), ScheduledJobInput{
		Name: "x", Prompt: "p", Schedule: "* * * * *",
	})

	run1, err := svc.StartRun(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotNil(t, run1)

	run2, err := svc.StartRun(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Nil(t, run2, "second concurrent enqueue must be dropped")
}

func TestScheduledJobService_MarkRunning_Idempotent(t *testing.T) {
	db := newSJTestDB(t)
	svc := NewScheduledJobService(db)
	job, _ := svc.Create(context.Background(), ScheduledJobInput{
		Name: "y", Prompt: "p", Schedule: "* * * * *",
	})
	run, _ := svc.StartRun(context.Background(), job.ID)

	ok, err := svc.MarkRunning(context.Background(), run.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = svc.MarkRunning(context.Background(), run.ID)
	require.NoError(t, err)
	assert.False(t, ok, "second mark-running must be a no-op")
}

func TestScheduledJobService_FailOrphans(t *testing.T) {
	svc := NewScheduledJobService(newSJTestDB(t))
	job, _ := svc.Create(context.Background(), ScheduledJobInput{
		Name: "z", Prompt: "p", Schedule: "* * * * *",
	})
	_, _ = svc.StartRun(context.Background(), job.ID)

	count, err := svc.FailOrphanedRuns(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
