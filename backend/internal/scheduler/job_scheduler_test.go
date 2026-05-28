package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeRunner struct {
	mu         sync.Mutex
	calls      int32
	hold       chan struct{} // close to release; nil = return immediately
	returnErr  error
	resultText string
}

func (f *fakeRunner) RunOnce(ctx context.Context, job *models.ScheduledJob) (*AgentRunResult, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.hold != nil {
		select {
		case <-f.hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	return &AgentRunResult{Text: f.resultText, Duration: 1}, nil
}

var schedTestDBCounter int32

func newSchedTestDB(t *testing.T) (*gorm.DB, *services.ScheduledJobService) {
	// Use a unique shared-memory DB name per call so parallel tests don't collide.
	name := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), atomic.AddInt32(&schedTestDBCounter, 1))
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ScheduledJob{}, &models.ScheduledJobRun{}, &models.EventLog{}))
	return db, services.NewScheduledJobService(db)
}

func TestJobScheduler_EnqueueRunsTerminates(t *testing.T) {
	db, sjSvc := newSchedTestDB(t)
	runner := &fakeRunner{resultText: "done"}
	sched := NewJobScheduler(db, sjSvc, runner, services.NewEventLogService(db),
		Options{MaxConcurrent: 1, Timeout: 2 * time.Second})

	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "t", Prompt: "go", Schedule: "* * * * *",
	})

	require.NoError(t, sched.Boot(context.Background()))
	defer sched.Stop(context.Background())

	run, err := sched.EnqueueOnce(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotNil(t, run)

	// Wait for terminal state
	require.Eventually(t, func() bool {
		r, _ := sjSvc.GetRun(context.Background(), run.ID)
		return r != nil && r.Status == models.JobRunCompleted
	}, 3*time.Second, 50*time.Millisecond)
}

func TestJobScheduler_DedupSecondEnqueue(t *testing.T) {
	db, sjSvc := newSchedTestDB(t)
	runner := &fakeRunner{hold: make(chan struct{})} // hangs until released
	sched := NewJobScheduler(db, sjSvc, runner, services.NewEventLogService(db),
		Options{MaxConcurrent: 2, Timeout: 5 * time.Second})

	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "x", Prompt: "p", Schedule: "* * * * *",
	})
	require.NoError(t, sched.Boot(context.Background()))
	defer func() {
		close(runner.hold)
		sched.Stop(context.Background())
	}()

	run1, err := sched.EnqueueOnce(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotNil(t, run1)

	// While run1 is hung in fakeRunner, second enqueue must be skipped.
	run2, err := sched.EnqueueOnce(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Nil(t, run2)
}

func TestJobScheduler_KillRun(t *testing.T) {
	db, sjSvc := newSchedTestDB(t)
	runner := &fakeRunner{hold: make(chan struct{})}
	sched := NewJobScheduler(db, sjSvc, runner, services.NewEventLogService(db),
		Options{MaxConcurrent: 1, Timeout: 30 * time.Second})

	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "k", Prompt: "p", Schedule: "* * * * *",
	})
	require.NoError(t, sched.Boot(context.Background()))
	defer sched.Stop(context.Background())

	run, _ := sched.EnqueueOnce(context.Background(), job.ID)
	// Wait for runner to start
	require.Eventually(t, func() bool { return atomic.LoadInt32(&runner.calls) > 0 },
		2*time.Second, 25*time.Millisecond)

	killed, err := sched.KillRun(context.Background(), run.ID)
	require.NoError(t, err)
	assert.True(t, killed)

	// Releasing the hold should not flip the row off failed.
	close(runner.hold)
	require.Eventually(t, func() bool {
		r, _ := sjSvc.GetRun(context.Background(), run.ID)
		return r != nil && r.Status == models.JobRunFailed && r.Error == "Job killed by user"
	}, 2*time.Second, 25*time.Millisecond)
}

func TestJobScheduler_BootRecoversOrphans(t *testing.T) {
	db, sjSvc := newSchedTestDB(t)
	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "o", Prompt: "p", Schedule: "* * * * *",
	})
	// Simulate an orphan: insert a queued run, then boot a new scheduler.
	_, _ = sjSvc.StartRun(context.Background(), job.ID)

	sched := NewJobScheduler(db, sjSvc, &fakeRunner{}, services.NewEventLogService(db),
		Options{MaxConcurrent: 1, Timeout: 1 * time.Second})
	require.NoError(t, sched.Boot(context.Background()))
	defer sched.Stop(context.Background())

	runs, _ := sjSvc.ListRuns(context.Background(), job.ID, 10, 0)
	require.Len(t, runs, 1)
	assert.Equal(t, models.JobRunFailed, runs[0].Status)
	assert.Equal(t, "Server restarted during execution", runs[0].Error)
}

func TestJobScheduler_TimeoutMarksRunTimedOut(t *testing.T) {
	db, sjSvc := newSchedTestDB(t)
	// Runner that holds forever — only ctx cancellation gets it out.
	runner := &fakeRunner{hold: make(chan struct{})}
	sched := NewJobScheduler(db, sjSvc, runner, services.NewEventLogService(db),
		Options{MaxConcurrent: 1, Timeout: 200 * time.Millisecond})

	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "to", Prompt: "p", Schedule: "* * * * *",
	})
	require.NoError(t, sched.Boot(context.Background()))
	defer func() {
		close(runner.hold)
		sched.Stop(context.Background())
	}()

	run, _ := sched.EnqueueOnce(context.Background(), job.ID)
	require.Eventually(t, func() bool {
		r, _ := sjSvc.GetRun(context.Background(), run.ID)
		return r != nil && r.Status == models.JobRunTimedOut
	}, 2*time.Second, 25*time.Millisecond)
}

var _ = errors.New // keep import even if not yet used
