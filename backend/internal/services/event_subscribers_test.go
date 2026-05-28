package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newEvTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.EventLog{}))
	return db
}

func TestEventLog_Subscribe_HandlerInvoked(t *testing.T) {
	svc := NewEventLogService(newEvTestDB(t))
	var calls int32

	svc.Subscribe("scheduled_job_completed", func(ctx context.Context, e EventEnvelope) {
		atomic.AddInt32(&calls, 1)
		assert.Equal(t, "scheduled_job_completed", e.Event)
	})

	require.NoError(t, svc.LogEvent(context.Background(), "scheduled_job_completed",
		map[string]any{"jobId": 1, "runId": 1}, nil))

	require.Eventually(t, func() bool { return atomic.LoadInt32(&calls) == 1 },
		1*time.Second, 25*time.Millisecond)
}

func TestEventLog_Subscribe_UnknownTypeIgnored(t *testing.T) {
	svc := NewEventLogService(newEvTestDB(t))
	var calls int32
	svc.Subscribe("type-a", func(_ context.Context, _ EventEnvelope) {
		atomic.AddInt32(&calls, 1)
	})
	_ = svc.LogEvent(context.Background(), "type-b", nil, nil)
	assert.Never(t, func() bool { return atomic.LoadInt32(&calls) > 0 },
		300*time.Millisecond, 25*time.Millisecond)
}

func TestEventLog_Subscribe_SlowHandlerDoesNotBlock(t *testing.T) {
	svc := NewEventLogService(newEvTestDB(t))
	hold := make(chan struct{})
	svc.Subscribe("e", func(_ context.Context, _ EventEnvelope) { <-hold })

	start := time.Now()
	require.NoError(t, svc.LogEvent(context.Background(), "e", nil, nil))
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 200*time.Millisecond, "LogEvent must not block on slow handlers")
	close(hold)
}
