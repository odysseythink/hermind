# Scheduled Jobs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist user-defined cron-scheduled agent runs with run history, at-most-one-in-flight dedup, kill semantics, and orphan recovery, plus a "continue in thread" UX.

**Architecture:** Two new GORM models (`ScheduledJob`, `ScheduledJobRun`) + a `JobScheduler` service that wraps `robfig/cron/v3` for dynamic registration + uses a `chan struct{}` semaphore for global concurrency control + a `map[int][]context.CancelFunc` for kill. Runs execute as goroutines that delegate to the existing agent `Runtime` with an auto-approve `ToolApproval` callback. Dedup is transactional at the DB layer. Orphan recovery flips non-terminal rows to `failed` on boot.

**Tech Stack:** Go 1.23, Gin, GORM (sqlite/postgres), `github.com/robfig/cron/v3`, testify.

**Source design:** `.gpowers/designs/2026-05-28-scheduled-jobs-design.md`

---

## File Structure

### Created
- `backend/internal/models/scheduled_job.go` — `ScheduledJob` GORM model
- `backend/internal/models/scheduled_job_run.go` — `ScheduledJobRun` GORM model
- `backend/internal/services/scheduled_job_service.go` — CRUD + dedup
- `backend/internal/services/scheduled_job_service_test.go`
- `backend/internal/scheduler/job_scheduler.go` — runtime cron registry, semaphore, kill, orphan recovery
- `backend/internal/scheduler/job_scheduler_test.go`
- `backend/internal/scheduler/run_executor.go` — `runOnce(ctx, jobID, runID)` agent dispatch
- `backend/internal/scheduler/run_executor_test.go`
- `backend/internal/scheduler/agent_runner.go` — `AgentRunner` interface (decouples from agent runtime for testability)
- `backend/internal/handlers/scheduled_jobs.go` — 10 routes
- `backend/internal/handlers/scheduled_jobs_test.go`
- `backend/internal/services/scheduled_job_continue.go` — continueInThread service
- `backend/internal/services/scheduled_job_continue_test.go`

### Modified
- `backend/internal/services/db.go` — append two new models to `AutoMigrate` (after `&models.PromptHistory{}`)
- `backend/cmd/server/main.go` — build `JobScheduler` and wire routes after services are up
- `backend/internal/config/config.go` — add `SCHEDULED_JOB_MAX_CONCURRENT`, `SCHEDULED_JOB_TIMEOUT_MS`, `SCHEDULED_JOB_MAX_ACTIVE` env bindings
- `backend/internal/services/event_log_service.go` — no change (Plan D adds pub/sub; here we just use `LogEvent`)

---

# PR1 · Schema + CRUD service (~150 LoC)

## Task 1: Define ScheduledJob and ScheduledJobRun models

**Files:**
- Create: `backend/internal/models/scheduled_job.go`
- Create: `backend/internal/models/scheduled_job_run.go`
- Modify: `backend/internal/services/db.go` (append to AutoMigrate list)

- [ ] **Step 1: Write ScheduledJob model**

```go
package models

import "time"

type ScheduledJob struct {
	ID        int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string     `gorm:"not null" json:"name"`
	Prompt    string     `gorm:"type:text;not null" json:"prompt"`
	Tools     string     `gorm:"type:text" json:"tools"` // JSON array; empty string = use defaults
	Schedule  string     `gorm:"not null" json:"schedule"` // cron expression
	Enabled   bool       `gorm:"default:true" json:"enabled"`
	LastRunAt *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt *time.Time `json:"nextRunAt,omitempty"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (ScheduledJob) TableName() string { return "scheduled_jobs" }
```

- [ ] **Step 2: Write ScheduledJobRun model**

```go
package models

import "time"

// Run statuses. Non-terminal: queued, running.
const (
	JobRunQueued    = "queued"
	JobRunRunning   = "running"
	JobRunCompleted = "completed"
	JobRunFailed    = "failed"
	JobRunTimedOut  = "timed_out"
)

type ScheduledJobRun struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID       int        `gorm:"index;not null" json:"jobId"`
	Status      string     `gorm:"default:queued;index" json:"status"`
	Result      string     `gorm:"type:text" json:"result"` // JSON
	Error       string     `gorm:"type:text" json:"error"`
	StartedAt   time.Time  `gorm:"autoCreateTime" json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	ReadAt      *time.Time `json:"readAt,omitempty"`

	Job *ScheduledJob `gorm:"foreignKey:JobID;constraint:OnDelete:CASCADE" json:"job,omitempty"`
}

func (ScheduledJobRun) TableName() string { return "scheduled_job_runs" }
```

- [ ] **Step 3: Register in AutoMigrate**

In `backend/internal/services/db.go`, append to the `AutoMigrate` call after `&models.PromptHistory{}`:

```go
&models.ScheduledJob{},
&models.ScheduledJobRun{},
```

- [ ] **Step 4: Build**

```bash
cd backend && go build ./...
```

Expected: no errors.

## Task 2: ScheduledJobService — failing tests first

**Files:**
- Create: `backend/internal/services/scheduled_job_service_test.go`

- [ ] **Step 1: Write failing test for Create + cron validation**

```go
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
```

- [ ] **Step 2: Run — expect compile errors**

```bash
cd backend && go test ./internal/services/ -run TestScheduledJobService
```

Expected: undefined `NewScheduledJobService`, `ScheduledJobInput`, `ErrInvalidCron`.

## Task 3: Implement ScheduledJobService

**Files:**
- Create: `backend/internal/services/scheduled_job_service.go`

- [ ] **Step 1: Write the service**

```go
package services

import (
	"context"
	"errors"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var (
	ErrInvalidCron     = errors.New("invalid cron expression")
	ErrJobNotFound     = errors.New("scheduled job not found")
	errAlreadyInflight = errors.New("scheduled job already has a run in flight")
)

type ScheduledJobService struct {
	db *gorm.DB
}

func NewScheduledJobService(db *gorm.DB) *ScheduledJobService {
	return &ScheduledJobService{db: db}
}

type ScheduledJobInput struct {
	Name     string
	Prompt   string
	Tools    string // JSON array; empty = defaults
	Schedule string
}

func parseCron(expr string) (cron.Schedule, error) {
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return nil, ErrInvalidCron
	}
	return sched, nil
}

func computeNextRunAt(expr string) *time.Time {
	sched, err := parseCron(expr)
	if err != nil {
		return nil
	}
	t := sched.Next(time.Now())
	return &t
}

func (s *ScheduledJobService) Create(ctx context.Context, in ScheduledJobInput) (*models.ScheduledJob, error) {
	if _, err := parseCron(in.Schedule); err != nil {
		return nil, err
	}
	job := &models.ScheduledJob{
		Name:      in.Name,
		Prompt:    in.Prompt,
		Tools:     in.Tools,
		Schedule:  in.Schedule,
		Enabled:   true,
		NextRunAt: computeNextRunAt(in.Schedule),
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

type UpdateJobInput struct {
	Name     *string
	Prompt   *string
	Tools    *string
	Schedule *string
	Enabled  *bool
}

func (s *ScheduledJobService) Update(ctx context.Context, id int, in UpdateJobInput) (*models.ScheduledJob, error) {
	if in.Schedule != nil {
		if _, err := parseCron(*in.Schedule); err != nil {
			return nil, err
		}
	}
	updates := map[string]any{}
	if in.Name != nil {
		updates["name"] = *in.Name
	}
	if in.Prompt != nil {
		updates["prompt"] = *in.Prompt
	}
	if in.Tools != nil {
		updates["tools"] = *in.Tools
	}
	if in.Schedule != nil {
		updates["schedule"] = *in.Schedule
		updates["next_run_at"] = computeNextRunAt(*in.Schedule)
	}
	if in.Enabled != nil {
		updates["enabled"] = *in.Enabled
	}
	if err := s.db.WithContext(ctx).Model(&models.ScheduledJob{}).
		Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *ScheduledJobService) Get(ctx context.Context, id int) (*models.ScheduledJob, error) {
	var job models.ScheduledJob
	if err := s.db.WithContext(ctx).First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, err
	}
	return &job, nil
}

func (s *ScheduledJobService) List(ctx context.Context) ([]models.ScheduledJob, error) {
	var jobs []models.ScheduledJob
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&jobs).Error
	return jobs, err
}

func (s *ScheduledJobService) AllEnabled(ctx context.Context) ([]models.ScheduledJob, error) {
	var jobs []models.ScheduledJob
	err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&jobs).Error
	return jobs, err
}

func (s *ScheduledJobService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.ScheduledJob{}, id).Error
}

// StartRun transactionally claims a queued row. Returns (nil, nil) if another
// run is already in flight for this job (dedup).
func (s *ScheduledJobService) StartRun(ctx context.Context, jobID int) (*models.ScheduledJobRun, error) {
	var run *models.ScheduledJobRun
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&models.ScheduledJobRun{}).
			Where("job_id = ? AND status IN ?", jobID, []string{models.JobRunQueued, models.JobRunRunning}).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errAlreadyInflight
		}
		row := &models.ScheduledJobRun{JobID: jobID, Status: models.JobRunQueued}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		run = row
		return nil
	})
	if errors.Is(err, errAlreadyInflight) {
		return nil, nil
	}
	return run, err
}

// MarkRunning transitions queued -> running. Returns true if the row was
// transitioned; false if it was already in a terminal (or different) state.
func (s *ScheduledJobService) MarkRunning(ctx context.Context, runID int) (bool, error) {
	res := s.db.WithContext(ctx).Model(&models.ScheduledJobRun{}).
		Where("id = ? AND status = ?", runID, models.JobRunQueued).
		Updates(map[string]any{
			"status":     models.JobRunRunning,
			"started_at": time.Now(),
		})
	return res.RowsAffected > 0, res.Error
}

func (s *ScheduledJobService) Complete(ctx context.Context, runID int, resultJSON string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.ScheduledJobRun{}).
		Where("id = ?", runID).
		Updates(map[string]any{
			"status":       models.JobRunCompleted,
			"result":       resultJSON,
			"completed_at": &now,
		}).Error
}

func (s *ScheduledJobService) failNonTerminal(ctx context.Context, runID int, status, msg string) (bool, error) {
	now := time.Now()
	res := s.db.WithContext(ctx).Model(&models.ScheduledJobRun{}).
		Where("id = ? AND status IN ?", runID, []string{models.JobRunQueued, models.JobRunRunning}).
		Updates(map[string]any{
			"status":       status,
			"error":        msg,
			"completed_at": &now,
		})
	return res.RowsAffected > 0, res.Error
}

func (s *ScheduledJobService) Fail(ctx context.Context, runID int, msg string) (bool, error) {
	return s.failNonTerminal(ctx, runID, models.JobRunFailed, msg)
}

func (s *ScheduledJobService) Timeout(ctx context.Context, runID int) (bool, error) {
	return s.failNonTerminal(ctx, runID, models.JobRunTimedOut, "Job execution timed out")
}

func (s *ScheduledJobService) Kill(ctx context.Context, runID int) (bool, error) {
	now := time.Now()
	res := s.db.WithContext(ctx).Model(&models.ScheduledJobRun{}).
		Where("id = ? AND status IN ?", runID, []string{models.JobRunQueued, models.JobRunRunning}).
		Updates(map[string]any{
			"status":       models.JobRunFailed,
			"error":        "Job killed by user",
			"completed_at": &now,
			"read_at":      &now,
		})
	return res.RowsAffected > 0, res.Error
}

// FailOrphanedRuns marks all non-terminal rows as failed. Call on boot.
func (s *ScheduledJobService) FailOrphanedRuns(ctx context.Context) (int64, error) {
	now := time.Now()
	res := s.db.WithContext(ctx).Model(&models.ScheduledJobRun{}).
		Where("status IN ?", []string{models.JobRunQueued, models.JobRunRunning}).
		Updates(map[string]any{
			"status":       models.JobRunFailed,
			"error":        "Server restarted during execution",
			"completed_at": &now,
		})
	return res.RowsAffected, res.Error
}

func (s *ScheduledJobService) ListRuns(ctx context.Context, jobID int, limit, offset int) ([]models.ScheduledJobRun, error) {
	var rows []models.ScheduledJobRun
	q := s.db.WithContext(ctx).Where("job_id = ?", jobID).Order("started_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	err := q.Find(&rows).Error
	return rows, err
}

func (s *ScheduledJobService) GetRun(ctx context.Context, runID int) (*models.ScheduledJobRun, error) {
	var row models.ScheduledJobRun
	if err := s.db.WithContext(ctx).Preload("Job").First(&row, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *ScheduledJobService) MarkRunRead(ctx context.Context, runID int) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.ScheduledJobRun{}).
		Where("id = ?", runID).Update("read_at", &now).Error
}

// UpdateRunTimestamps recomputes nextRunAt + sets lastRunAt to now. Called by
// scheduler when a run is enqueued.
func (s *ScheduledJobService) UpdateRunTimestamps(ctx context.Context, jobID int) error {
	job, err := s.Get(ctx, jobID)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.ScheduledJob{}).
		Where("id = ?", jobID).Updates(map[string]any{
			"last_run_at": &now,
			"next_run_at": computeNextRunAt(job.Schedule),
		}).Error
}
```

- [ ] **Step 2: Run tests — expect PASS**

```bash
cd backend && go test ./internal/services/ -run TestScheduledJobService -v
```

Expected: 5 tests PASS.

## Task 4: Commit PR1

- [ ] **Step 1: Stage and commit**

```bash
git add backend/internal/models/scheduled_job.go \
        backend/internal/models/scheduled_job_run.go \
        backend/internal/services/db.go \
        backend/internal/services/scheduled_job_service.go \
        backend/internal/services/scheduled_job_service_test.go
git commit -m "$(cat <<'EOF'
feat(scheduled-jobs): add ScheduledJob + ScheduledJobRun models and CRUD

Defines the persistence layer for cron-scheduled agent runs. Service
methods enforce transactional dedup (at most one queued|running row per
job), provide kill/timeout transitions, and a boot-time orphan recovery
helper (FailOrphanedRuns).

No scheduler / runtime integration yet — that lands in PR2.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

# PR2 · JobScheduler core (~400 LoC)

## Task 5: Define AgentRunner interface

**Files:**
- Create: `backend/internal/scheduler/agent_runner.go`

- [ ] **Step 1: Write the interface**

```go
package scheduler

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
)

// AgentRunResult carries the result of an agent run for persistence.
type AgentRunResult struct {
	Text      string         `json:"text"`
	Thoughts  []any          `json:"thoughts,omitempty"`
	ToolCalls []any          `json:"toolCalls,omitempty"`
	Outputs   []any          `json:"outputs,omitempty"`
	Metrics   map[string]any `json:"metrics,omitempty"`
	Duration  int64          `json:"duration"` // milliseconds
}

// AgentRunner abstracts the agent invocation so scheduler is testable without
// the real Runtime. Implementations must respect ctx.Done() for cancellation
// (kill semantics).
//
// Auto-approve: when called via the scheduler the implementation MUST treat
// all internal tool-approval requests as approved, recording each one to the
// EventLog under "scheduled_job_tool_auto_approved" for audit.
type AgentRunner interface {
	RunOnce(ctx context.Context, job *models.ScheduledJob) (*AgentRunResult, error)
}
```

## Task 6: JobScheduler skeleton + tests for dedup + kill + orphan

**Files:**
- Create: `backend/internal/scheduler/job_scheduler.go`
- Create: `backend/internal/scheduler/job_scheduler_test.go`

- [ ] **Step 1: Write failing tests using a fake AgentRunner**

```go
package scheduler

import (
	"context"
	"errors"
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

func newSchedTestDB(t *testing.T) (*gorm.DB, *services.ScheduledJobService) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
```

- [ ] **Step 2: Run — expect compile errors**

```bash
cd backend && go test ./internal/scheduler/
```

Expected: package not yet defined.

## Task 7: Implement JobScheduler

**Files:**
- Create: `backend/internal/scheduler/job_scheduler.go`

- [ ] **Step 1: Write the scheduler**

```go
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/mlog"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Options struct {
	MaxConcurrent int           // semaphore size; default 1
	Timeout       time.Duration // per-run wall-clock; default 10min
	MaxActive     int           // cap on enabled jobs (0 = unlimited)
}

func (o *Options) fill() {
	if o.MaxConcurrent <= 0 {
		o.MaxConcurrent = 1
	}
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Minute
	}
}

type JobScheduler struct {
	db       *gorm.DB
	sjSvc    *services.ScheduledJobService
	runner   AgentRunner
	eventLog *services.EventLogService
	opts     Options

	cron *cron.Cron
	sem  chan struct{}

	mu          sync.Mutex
	entries     map[int]cron.EntryID
	inflightCxl map[int][]context.CancelFunc

	booted bool
	wg     sync.WaitGroup
}

func NewJobScheduler(
	db *gorm.DB,
	sjSvc *services.ScheduledJobService,
	runner AgentRunner,
	eventLog *services.EventLogService,
	opts Options,
) *JobScheduler {
	opts.fill()
	return &JobScheduler{
		db:          db,
		sjSvc:       sjSvc,
		runner:      runner,
		eventLog:    eventLog,
		opts:        opts,
		cron:        cron.New(),
		sem:         make(chan struct{}, opts.MaxConcurrent),
		entries:     make(map[int]cron.EntryID),
		inflightCxl: make(map[int][]context.CancelFunc),
	}
}

// Boot recovers orphaned runs then registers all enabled jobs.
func (s *JobScheduler) Boot(ctx context.Context) error {
	if s.booted {
		return nil
	}
	if _, err := s.sjSvc.FailOrphanedRuns(ctx); err != nil {
		return fmt.Errorf("fail orphans: %w", err)
	}
	jobs, err := s.sjSvc.AllEnabled(ctx)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		s.addCron(j)
	}
	s.cron.Start()
	s.booted = true
	mlog.Info("scheduler started", mlog.Int("enabled-jobs", len(jobs)))
	return nil
}

func (s *JobScheduler) Stop(ctx context.Context) error {
	if !s.booted {
		return nil
	}
	cronCtx := s.cron.Stop()
	done := make(chan struct{})
	go func() {
		<-cronCtx.Done()
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
	s.booted = false
	return nil
}

// SyncJob is called by the CRUD service after a row changes. It removes any
// existing cron entry and re-adds if the new row is enabled.
func (s *JobScheduler) SyncJob(ctx context.Context, jobID int) error {
	s.removeCron(jobID)
	job, err := s.sjSvc.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Enabled {
		s.addCron(*job)
	}
	return nil
}

func (s *JobScheduler) RemoveJob(ctx context.Context, jobID int) {
	s.removeCron(jobID)
	s.killAllForJob(jobID)
}

// EnqueueOnce is the manual-trigger path. Returns (nil, nil) if a run is in
// flight. Cron firings also go through this path.
func (s *JobScheduler) EnqueueOnce(ctx context.Context, jobID int) (*models.ScheduledJobRun, error) {
	run, err := s.sjSvc.StartRun(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, nil
	}
	_ = s.sjSvc.UpdateRunTimestamps(ctx, jobID)
	s.wg.Add(1)
	go s.executeRun(jobID, run.ID)
	return run, nil
}

// KillRun marks a run failed and cancels its goroutine.
func (s *JobScheduler) KillRun(ctx context.Context, runID int) (bool, error) {
	row, err := s.sjSvc.GetRun(ctx, runID)
	if err != nil {
		return false, err
	}
	killed, err := s.sjSvc.Kill(ctx, runID)
	if err != nil || !killed {
		return false, err
	}
	s.cancelInflight(row.JobID)
	return true, nil
}

func (s *JobScheduler) addCron(job models.ScheduledJob) {
	id, err := s.cron.AddFunc(job.Schedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := s.EnqueueOnce(ctx, job.ID); err != nil {
			mlog.Warning("scheduler enqueue failed",
				mlog.Int("job", job.ID), mlog.String("err", err.Error()))
		}
	})
	if err != nil {
		mlog.Warning("scheduler addCron failed",
			mlog.Int("job", job.ID), mlog.String("err", err.Error()))
		return
	}
	s.mu.Lock()
	s.entries[job.ID] = id
	s.mu.Unlock()
}

func (s *JobScheduler) removeCron(jobID int) {
	s.mu.Lock()
	id, ok := s.entries[jobID]
	delete(s.entries, jobID)
	s.mu.Unlock()
	if ok {
		s.cron.Remove(id)
	}
}

func (s *JobScheduler) cancelInflight(jobID int) {
	s.mu.Lock()
	fns := s.inflightCxl[jobID]
	delete(s.inflightCxl, jobID)
	s.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func (s *JobScheduler) killAllForJob(jobID int) {
	s.cancelInflight(jobID)
}

func (s *JobScheduler) executeRun(jobID, runID int) {
	defer s.wg.Done()
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	// Acquire fresh context with timeout + cancel func
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.Timeout)
	defer cancel()
	s.mu.Lock()
	s.inflightCxl[jobID] = append(s.inflightCxl[jobID], cancel)
	s.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			_, _ = s.sjSvc.Fail(context.Background(), runID, fmt.Sprintf("panic: %v", r))
			mlog.Error("scheduled job panicked", mlog.Int("job", jobID), mlog.Any("recover", r))
		}
	}()

	// Mark running
	ok, err := s.sjSvc.MarkRunning(ctx, runID)
	if err != nil || !ok {
		return // either errored or row already terminal (kill arrived first)
	}

	job, err := s.sjSvc.Get(ctx, jobID)
	if err != nil {
		_, _ = s.sjSvc.Fail(context.Background(), runID, err.Error())
		return
	}

	start := time.Now()
	result, err := s.runner.RunOnce(ctx, job)
	duration := time.Since(start)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			_, _ = s.sjSvc.Timeout(context.Background(), runID)
			s.emitTerminal(jobID, runID, "scheduled_job_timed_out", err.Error())
			return
		}
		if errors.Is(err, context.Canceled) {
			// killed — DB already updated
			return
		}
		_, _ = s.sjSvc.Fail(context.Background(), runID, err.Error())
		s.emitTerminal(jobID, runID, "scheduled_job_failed", err.Error())
		return
	}
	if result == nil {
		result = &AgentRunResult{}
	}
	result.Duration = duration.Milliseconds()
	b, _ := json.Marshal(result)
	_ = s.sjSvc.Complete(context.Background(), runID, string(b))
	s.emitTerminal(jobID, runID, "scheduled_job_completed", "")
}

func (s *JobScheduler) emitTerminal(jobID, runID int, event, errMsg string) {
	if s.eventLog == nil {
		return
	}
	meta := map[string]any{"jobId": jobID, "runId": runID}
	if errMsg != "" {
		meta["error"] = errMsg
	}
	_ = s.eventLog.LogEvent(context.Background(), event, meta, nil)
}
```

- [ ] **Step 2: Run tests — expect PASS**

```bash
cd backend && go test ./internal/scheduler/ -v
```

Expected: 5 scheduler tests PASS within ~5s total.

## Task 8: Commit PR2

- [ ] **Step 1: Stage + commit**

```bash
git add backend/internal/scheduler/agent_runner.go \
        backend/internal/scheduler/job_scheduler.go \
        backend/internal/scheduler/job_scheduler_test.go
git commit -m "$(cat <<'EOF'
feat(scheduler): JobScheduler with cron + semaphore + kill

Adds an in-process scheduler that wraps robfig/cron/v3 for dynamic
registration, a chan-based semaphore for global concurrency control,
and a per-job cancelFunc map for kill semantics. Runs delegate to an
AgentRunner interface (mocked in tests). Boot-time orphan recovery
flips non-terminal rows from previous server lives to failed.

Wired to EventLogService — terminal states emit scheduled_job_completed
/ _failed / _timed_out rows for downstream consumers (Plan D WebPush).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

# PR3 · HTTP endpoints + tools catalog + manual trigger (~300 LoC)

## Task 9: Implement AgentRunner backed by the real agent Runtime

**Files:**
- Create: `backend/internal/scheduler/run_executor.go`
- Create: `backend/internal/scheduler/run_executor_test.go`

> Note: this task adapts to the agent runtime's actual API. Read first:
> ```bash
> grep -n 'CreateInvocation\|func.*Runtime' backend/internal/agent/*.go | head -10
> ```

- [ ] **Step 1: Write the adapter**

```go
package scheduler

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

// RuntimeAgentRunner adapts the existing agent.Runtime to AgentRunner.
//
// Auto-approve: this runner registers a workspace-less invocation and runs the
// prompt against the agent runtime with all tool approvals auto-granted (each
// approval logged to EventLog under "scheduled_job_tool_auto_approved").
type RuntimeAgentRunner struct {
	rt       *agent.Runtime
	eventLog *services.EventLogService
}

func NewRuntimeAgentRunner(rt *agent.Runtime, eventLog *services.EventLogService) *RuntimeAgentRunner {
	return &RuntimeAgentRunner{rt: rt, eventLog: eventLog}
}

func (r *RuntimeAgentRunner) RunOnce(ctx context.Context, job *models.ScheduledJob) (*AgentRunResult, error) {
	// IMPLEMENTATION NOTE (file the engineer should fill in based on the actual
	// Runtime API): the binding here calls into agent.Runtime to spin up an
	// ephemeral invocation, hand it job.Prompt and (if non-empty) the
	// JSON-decoded job.Tools as the tool override allow-list, then collects
	// the run's text + tool calls + outputs.
	//
	// Approval callback must:
	//   1. Auto-approve every request.
	//   2. Log "scheduled_job_tool_auto_approved" to r.eventLog with metadata
	//      {jobId, tool, payload-summary}.
	//
	// The exact wiring depends on agent.Runtime's public surface — open
	// invocation.go / runtime.go / handoff.go and pick the right entrypoint.
	// If a clean one doesn't exist yet, prefer extending Runtime with a
	// RunHeadless(ctx, prompt, toolAllowList, approveFn) method over a
	// duplicate runtime in this package.
	return nil, ErrRuntimeNotWired
}

var ErrRuntimeNotWired = struct{ error }{ // sentinel
	error: errStr("RuntimeAgentRunner.RunOnce not yet wired to agent.Runtime"),
}

type errStr string

func (e errStr) Error() string { return string(e) }
```

> **The RuntimeAgentRunner ships with `ErrRuntimeNotWired`.** That is intentional — wiring depends on Runtime internals not safe to guess. The next step opens those files and replaces the stub.

- [ ] **Step 2: Inspect the agent Runtime surface**

```bash
grep -n 'func.*Runtime\|RunHeadless\|RequestToolApproval' backend/internal/agent/runtime.go backend/internal/agent/handler.go backend/internal/agent/handoff.go 2>/dev/null
```

Read the matched files. Identify the entry point that (a) takes a prompt, (b) accepts a tool allow-list (or rejects everything not on a list), and (c) exposes the approval callback.

- [ ] **Step 3: Replace the stub**

Replace the body of `RunOnce` with a real call using whatever entry point you found. Add a comment in the file header naming the function you bound to. If no suitable entry exists, **add** `Runtime.RunHeadless(ctx, prompt, toolAllowList, approveFn) (*AgentRunResult, error)` to `backend/internal/agent/runtime.go` and call it from here.

> **Why this is two steps:** the agent runtime is an unfamiliar surface for this plan's author. The skill convention is "don't write code into territory you haven't read." Read first, then wire.

- [ ] **Step 4: Write a smoke test for the adapter using a mock Runtime if one exists**

If `backend/internal/agent/mockllm_test.go` provides a usable fake, write `TestRuntimeAgentRunner_RunOnce_Smoke` that creates a `*models.ScheduledJob{Prompt: "hello"}` and asserts:
- `result.Text` is non-empty
- the EventLog has at least zero `scheduled_job_tool_auto_approved` rows (depending on whether the prompt triggered a tool)
- no error

If no usable fake exists, skip this step — the integration test in Task 17 will exercise the full path.

## Task 10: ScheduledJobsHandler — failing tests for endpoints

**Files:**
- Create: `backend/internal/handlers/scheduled_jobs_test.go`

- [ ] **Step 1: Write tests covering all endpoints**

```go
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/scheduler"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type immediateRunner struct{}

func (immediateRunner) RunOnce(_ context.Context, _ *models.ScheduledJob) (*scheduler.AgentRunResult, error) {
	return &scheduler.AgentRunResult{Text: "ok"}, nil
}

func newSJHandlerEnv(t *testing.T) (*gin.Engine, *gorm.DB, *services.ScheduledJobService, *scheduler.JobScheduler) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ScheduledJob{}, &models.ScheduledJobRun{}, &models.EventLog{}, &models.User{}))
	sjSvc := services.NewScheduledJobService(db)
	evt := services.NewEventLogService(db)
	sched := scheduler.NewJobScheduler(db, sjSvc, immediateRunner{}, evt,
		scheduler.Options{MaxConcurrent: 1, Timeout: 1 * 1_000_000_000})
	require.NoError(t, sched.Boot(context.Background()))
	authSvc := services.NewAuthService(db, &config.Config{Secret: "t"})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: "admin"})
		c.Next()
	})
	api := r.Group("/api")
	RegisterScheduledJobsRoutes(api, sjSvc, sched, authSvc)
	return r, db, sjSvc, sched
}

func TestScheduledJobs_CreateListGetDelete(t *testing.T) {
	r, _, sjSvc, _ := newSJHandlerEnv(t)

	body, _ := json.Marshal(map[string]any{
		"name": "weekly", "prompt": "do thing", "schedule": "0 9 * * 1",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/scheduled-jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var created struct{ Job models.ScheduledJob }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.NotZero(t, created.Job.ID)

	// LIST
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/scheduled-jobs", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// DELETE
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/scheduled-jobs/"+strconv.Itoa(created.Job.ID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	_, err := sjSvc.Get(context.Background(), created.Job.ID)
	assert.Error(t, err)
}

func TestScheduledJobs_TriggerProducesRun(t *testing.T) {
	r, _, sjSvc, _ := newSJHandlerEnv(t)
	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "trigger", Prompt: "p", Schedule: "* * * * *",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/scheduled-jobs/"+strconv.Itoa(job.ID)+"/trigger", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	require.Eventually(t, func() bool {
		runs, _ := sjSvc.ListRuns(context.Background(), job.ID, 10, 0)
		return len(runs) > 0 && runs[0].Status == models.JobRunCompleted
	}, 2_000_000_000 /* 2s */, 50_000_000 /* 50ms */)
}

func TestScheduledJobs_InvalidCronReturns400(t *testing.T) {
	r, _, _, _ := newSJHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"name": "bad", "prompt": "p", "schedule": "🚫"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/scheduled-jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run — expect compile error**

```bash
cd backend && go test ./internal/handlers/ -run TestScheduledJobs
```

Expected: undefined `RegisterScheduledJobsRoutes`.

## Task 11: Implement endpoints

**Files:**
- Create: `backend/internal/handlers/scheduled_jobs.go`

- [ ] **Step 1: Write handler**

```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/scheduler"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type ScheduledJobsHandler struct {
	svc   *services.ScheduledJobService
	sched *scheduler.JobScheduler
}

func NewScheduledJobsHandler(svc *services.ScheduledJobService, sched *scheduler.JobScheduler) *ScheduledJobsHandler {
	return &ScheduledJobsHandler{svc: svc, sched: sched}
}

type createReq struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Tools    string `json:"tools"`
	Schedule string `json:"schedule"`
}

func (h *ScheduledJobsHandler) Create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	job, err := h.svc.Create(c.Request.Context(), services.ScheduledJobInput{
		Name: req.Name, Prompt: req.Prompt, Tools: req.Tools, Schedule: req.Schedule,
	})
	if err != nil {
		if errors.Is(err, services.ErrInvalidCron) {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cron"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	_ = h.sched.SyncJob(c.Request.Context(), job.ID)
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func (h *ScheduledJobsHandler) List(c *gin.Context) {
	jobs, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

type updateReq struct {
	Name     *string `json:"name"`
	Prompt   *string `json:"prompt"`
	Tools    *string `json:"tools"`
	Schedule *string `json:"schedule"`
	Enabled  *bool   `json:"enabled"`
}

func (h *ScheduledJobsHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	job, err := h.svc.Update(c.Request.Context(), id, services.UpdateJobInput{
		Name: req.Name, Prompt: req.Prompt, Tools: req.Tools,
		Schedule: req.Schedule, Enabled: req.Enabled,
	})
	if err != nil {
		if errors.Is(err, services.ErrInvalidCron) {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cron"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	_ = h.sched.SyncJob(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func (h *ScheduledJobsHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	h.sched.RemoveJob(c.Request.Context(), id)
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ScheduledJobsHandler) Trigger(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	run, err := h.sched.EnqueueOnce(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if run == nil {
		c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "already in flight"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run": run})
}

func (h *ScheduledJobsHandler) ListRuns(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	runs, err := h.svc.ListRuns(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func (h *ScheduledJobsHandler) KillRun(c *gin.Context) {
	runID, err := strconv.Atoi(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	killed, err := h.sched.KillRun(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": killed})
}

func (h *ScheduledJobsHandler) MarkRunRead(c *gin.Context) {
	runID, err := strconv.Atoi(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	if err := h.svc.MarkRunRead(c.Request.Context(), runID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterScheduledJobsRoutes(r *gin.RouterGroup, svc *services.ScheduledJobService, sched *scheduler.JobScheduler, authSvc *services.AuthService) {
	h := NewScheduledJobsHandler(svc, sched)
	g := r.Group("/scheduled-jobs", middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}))
	g.GET("", h.List)
	g.POST("", h.Create)
	g.PATCH("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/trigger", h.Trigger)
	g.GET("/:id/runs", h.ListRuns)
	g.POST("/runs/:runId/kill", h.KillRun)
	g.POST("/runs/:runId/read", h.MarkRunRead)
}
```

- [ ] **Step 2: Run tests — expect PASS**

```bash
cd backend && go test ./internal/handlers/ -run TestScheduledJobs -v
```

Expected: 3 tests PASS.

## Task 12: Commit PR3

- [ ] **Step 1: Stage + commit**

```bash
git add backend/internal/handlers/scheduled_jobs.go \
        backend/internal/handlers/scheduled_jobs_test.go \
        backend/internal/scheduler/run_executor.go
git commit -m "$(cat <<'EOF'
feat(scheduled-jobs): 8 HTTP endpoints + RuntimeAgentRunner adapter

Admin-only CRUD plus trigger/kill/mark-read endpoints. Trigger routes
through JobScheduler.EnqueueOnce which dedups (returns 409 if a run is
already in flight). RuntimeAgentRunner adapts the agent.Runtime to the
scheduler's AgentRunner interface — auto-approves tool calls and logs
each one to EventLog for audit.

Tools catalog endpoint (/scheduled-jobs/tools) and continueInThread land
in PR4.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

# PR4 · continueInThread + tools catalog (~250 LoC)

## Task 13: continueInThread service

**Files:**
- Create: `backend/internal/services/scheduled_job_continue.go`
- Create: `backend/internal/services/scheduled_job_continue_test.go`

- [ ] **Step 1: Failing test first**

```go
package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContinueInThread_CreatesWorkspaceAndThread(t *testing.T) {
	db := newSJTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}, &models.WorkspaceThread{}, &models.WorkspaceChat{}, &models.WorkspaceUser{}))

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
```

- [ ] **Step 2: Implement**

```go
package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

type ScheduledJobContinueService struct {
	db   *gorm.DB
	sjSvc *ScheduledJobService
}

func NewScheduledJobContinueService(db *gorm.DB, sjSvc *ScheduledJobService) *ScheduledJobContinueService {
	return &ScheduledJobContinueService{db: db, sjSvc: sjSvc}
}

func (s *ScheduledJobContinueService) ContinueInThread(ctx context.Context, runID int) (*models.Workspace, *models.WorkspaceThread, error) {
	run, err := s.sjSvc.GetRun(ctx, runID)
	if err != nil {
		return nil, nil, err
	}

	var parsed struct {
		Text    string `json:"text"`
		Sources []any  `json:"sources,omitempty"`
		Outputs []any  `json:"outputs,omitempty"`
	}
	_ = json.Unmarshal([]byte(run.Result), &parsed)
	if parsed.Text == "" {
		parsed.Text = "No response was generated."
	}

	// Upsert workspace by slug "scheduled-jobs"
	var ws models.Workspace
	if err := s.db.WithContext(ctx).Where("slug = ?", "scheduled-jobs").First(&ws).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, nil, err
		}
		ws = models.Workspace{
			Name: "Scheduled Jobs", Slug: "scheduled-jobs",
			ChatMode: ptrStr("automatic"),
			CreatedAt: time.Now(), LastUpdatedAt: time.Now(),
		}
		if err := s.db.WithContext(ctx).Create(&ws).Error; err != nil {
			return nil, nil, err
		}
	}

	thr := models.WorkspaceThread{
		WorkspaceID: ws.ID,
		Slug:        uuid.NewString()[:8],
		Name:        "Run #" + strconvItoa(run.ID),
		CreatedAt:   time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&thr).Error; err != nil {
		return nil, nil, err
	}

	resp := map[string]any{
		"text": parsed.Text, "sources": parsed.Sources, "outputs": parsed.Outputs, "type": "chat",
	}
	respJSON, _ := json.Marshal(resp)
	chat := models.WorkspaceChat{
		WorkspaceID: ws.ID,
		ThreadID:    &thr.ID,
		Prompt:      run.Job.Prompt,
		Response:    string(respJSON),
		Include:     true,
		CreatedAt:   time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&chat).Error; err != nil {
		return nil, nil, err
	}
	return &ws, &thr, nil
}

func ptrStr(s string) *string { return &s }
```

> The `strconvItoa` reference assumes `strconv` is in the package's import set or you re-export `strconv.Itoa`. The plan author left this stub because the file's other helpers may already have `strconv` — verify imports and either:
>
> ```go
> import "strconv"
> // ...
> Name: "Run #" + strconv.Itoa(run.ID),
> ```
>
> or add a tiny local helper. Don't ship `strconvItoa` as-is.

- [ ] **Step 3: Run tests — expect PASS**

```bash
cd backend && go test ./internal/services/ -run TestContinueInThread_CreatesWorkspaceAndThread -v
```

## Task 14: continueInThread endpoint + commit PR4

**Files:**
- Modify: `backend/internal/handlers/scheduled_jobs.go`

- [ ] **Step 1: Add Continue method on handler**

Append to `scheduled_jobs.go`:

```go
type ContinueHandlerDeps struct {
	cont *services.ScheduledJobContinueService
}

func (h *ScheduledJobsHandler) Continue(c *gin.Context, cont *services.ScheduledJobContinueService) {
	runID, err := strconv.Atoi(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	ws, thr, err := cont.ContinueInThread(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "thread": thr})
}
```

Update `RegisterScheduledJobsRoutes` to accept a `*ScheduledJobContinueService` and register the route. (Easiest: make the continue service a field on `ScheduledJobsHandler`.)

```go
type ScheduledJobsHandler struct {
	svc    *services.ScheduledJobService
	sched  *scheduler.JobScheduler
	contSvc *services.ScheduledJobContinueService
}

func NewScheduledJobsHandler(svc *services.ScheduledJobService, sched *scheduler.JobScheduler, contSvc *services.ScheduledJobContinueService) *ScheduledJobsHandler {
	return &ScheduledJobsHandler{svc: svc, sched: sched, contSvc: contSvc}
}

// inside RegisterScheduledJobsRoutes, after KillRun:
g.POST("/runs/:runId/continue", h.Continue)
```

And update the existing `Continue` body to use `h.contSvc` instead of the parameter.

- [ ] **Step 2: Update test helper to construct the new signature**

In `scheduled_jobs_test.go`, where the handler is built, change to:

```go
contSvc := services.NewScheduledJobContinueService(db, sjSvc)
RegisterScheduledJobsRoutes(api, sjSvc, sched, contSvc, authSvc)
```

- [ ] **Step 3: Add an end-to-end test**

```go
func TestScheduledJobs_ContinueInThread_Endpoint(t *testing.T) {
	r, db, sjSvc, _ := newSJHandlerEnv(t)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceThread{}))
	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "c", Prompt: "P", Schedule: "* * * * *",
	})
	run, _ := sjSvc.StartRun(context.Background(), job.ID)
	_ = sjSvc.Complete(context.Background(), run.ID, `{"text":"answer"}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/scheduled-jobs/runs/"+strconv.Itoa(run.ID)+"/continue", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
```

- [ ] **Step 4: Run all scheduler/handler tests**

```bash
cd backend && go test ./internal/scheduler/ ./internal/handlers/ ./internal/services/ -v 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 5: Commit PR4**

```bash
git add backend/internal/services/scheduled_job_continue.go \
        backend/internal/services/scheduled_job_continue_test.go \
        backend/internal/handlers/scheduled_jobs.go \
        backend/internal/handlers/scheduled_jobs_test.go
git commit -m "$(cat <<'EOF'
feat(scheduled-jobs): continueInThread service + endpoint

Adds POST /scheduled-jobs/runs/:runId/continue. Upserts a "Scheduled Jobs"
workspace (slug=scheduled-jobs, chatMode=automatic), creates a new thread,
and seeds a single WorkspaceChat with the run's prompt as user message
and the parsed result text as the response — matching anything-llm's
continueInThread behavior.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

# PR5 · main.go wiring + tools catalog endpoint + final smoke (~150 LoC)

## Task 15: Tools catalog endpoint

**Files:**
- Modify: `backend/internal/handlers/scheduled_jobs.go`

- [ ] **Step 1: Add the tools catalog handler**

The tools catalog enumerates what scheduled jobs are allowed to invoke. For Phase 2 we expose the same shape as anything-llm but populate only what backend currently registers via `agent/tools/builder.go`. Add this method:

```go
type ToolCatalogItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	RequiresSetup bool   `json:"requiresSetup,omitempty"`
}

type ToolCatalogCategory struct {
	Category string            `json:"category"`
	Name     string            `json:"name"`
	Items    []ToolCatalogItem `json:"items"`
}

func (h *ScheduledJobsHandler) ListTools(c *gin.Context) {
	// Static default skills first (always present).
	cats := []ToolCatalogCategory{
		{
			Category: "agent-skills", Name: "Agent Skills",
			Items: []ToolCatalogItem{
				{ID: "rag-memory", Name: "RAG Memory", Description: "Recall and cite information from embedded documents"},
				{ID: "document-summarizer", Name: "Document Summarizer", Description: "Summarize documents in the workspace"},
				{ID: "web-scraping", Name: "Web Scraping", Description: "Scrape content from web pages"},
				{ID: "create-chart", Name: "Create Charts", Description: "Generate data visualization charts"},
				{ID: "web-browsing", Name: "Web Browsing", Description: "Search and browse the web"},
				{ID: "sql-agent", Name: "SQL Agent", Description: "Query connected SQL databases"},
				{ID: "filesystem-agent", Name: "Filesystem"},
				{ID: "create-files-agent", Name: "Create Files"},
			},
		},
	}
	// MCP servers — left as a TODO for the engineer to populate from MCPService
	// if available. The shape:
	//   {Category: "mcp-servers", Name: "MCP Servers", Items: [{ID: "@@mcp_" + serverName, Name: ..., Description: ...}]}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}
```

Register the route inside `RegisterScheduledJobsRoutes`:

```go
g.GET("/tools", h.ListTools)
```

- [ ] **Step 2: Test that catalog returns categories**

Append to `scheduled_jobs_test.go`:

```go
func TestScheduledJobs_ListTools(t *testing.T) {
	r, _, _, _ := newSJHandlerEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/scheduled-jobs/tools", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct{ Categories []ToolCatalogCategory }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(t, body.Categories)
}
```

```bash
cd backend && go test ./internal/handlers/ -run TestScheduledJobs_ListTools -v
```

Expected: PASS.

## Task 16: Wire scheduler into main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add config bindings**

In `backend/internal/config/config.go` add:

```go
SchedJobMaxConcurrent int    `env:"SCHEDULED_JOB_MAX_CONCURRENT" envDefault:"1"`
SchedJobTimeoutMS    int    `env:"SCHEDULED_JOB_TIMEOUT_MS" envDefault:"600000"`
SchedJobMaxActive    int    `env:"SCHEDULED_JOB_MAX_ACTIVE" envDefault:"0"`
```

- [ ] **Step 2: Construct scheduler + register routes**

In `cmd/server/main.go`, after the agent runtime is built (search for `agent.NewRuntime` or similar), add:

```go
sjSvc := services.NewScheduledJobService(db)
agentRunner := scheduler.NewRuntimeAgentRunner(agentRuntime, eventLogSvc) // adapt names to actual locals
sched := scheduler.NewJobScheduler(db, sjSvc, agentRunner, eventLogSvc, scheduler.Options{
	MaxConcurrent: cfg.SchedJobMaxConcurrent,
	Timeout:       time.Duration(cfg.SchedJobTimeoutMS) * time.Millisecond,
	MaxActive:     cfg.SchedJobMaxActive,
})
if err := sched.Boot(ctx); err != nil {
	mlog.Fatal("scheduler boot failed", mlog.Err(err))
}
defer func() { _ = sched.Stop(context.Background()) }()

contSvc := services.NewScheduledJobContinueService(db, sjSvc)
handlers.RegisterScheduledJobsRoutes(api, sjSvc, sched, contSvc, authSvc)
```

(Adapt local variable names to what's in main.go — `eventLogSvc`, `agentRuntime`, etc.)

- [ ] **Step 3: Build**

```bash
cd backend && go build ./...
```

Expected: no errors.

## Task 17: End-to-end smoke against the real binary

- [ ] **Step 1: Start the server briefly**

```bash
cd backend && timeout 8 go run ./cmd/server/ 2>&1 | head -40
```

Look for:
- `scheduler started enabled-jobs=0`
- no panic

- [ ] **Step 2: Final commit**

```bash
git add backend/internal/handlers/scheduled_jobs.go \
        backend/internal/handlers/scheduled_jobs_test.go \
        backend/internal/config/config.go \
        backend/cmd/server/main.go
git commit -m "$(cat <<'EOF'
feat(scheduled-jobs): wire scheduler into main.go + tools catalog endpoint

Boots JobScheduler with env-tunable concurrency / timeout, mounts the
8 admin routes, and exposes GET /scheduled-jobs/tools returning the
shape anything-llm's frontend expects. MCP-server enumeration is left
as an extension point — wire to MCPService.servers() once that surface
stabilizes.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

| Spec § | Tasks |
|---|---|
| §3.1 Tables | 1 |
| §3.2 runtime (goroutine+sem) | 5, 6, 7 |
| §3.3 transactional dedup | 2, 3 (StartRun) |
| §3.4 kill semantics | 6 (test), 7 (KillRun + cancelInflight) |
| §3.5 auto-approve | 9, 11 (wired via AgentRunner) |
| §3.6 orphan recovery | 3 (FailOrphanedRuns), 7 (Boot calls it) |
| §4 endpoints | 10, 11, 14, 15 |
| §5 tools catalog | 15 |
| §6 continueInThread | 13, 14 |
| §7 config | 16 |
| §8 PR breakdown | PR1=Tasks 1-4, PR2=5-8, PR3=9-12, PR4=13-14, PR5=15-17 |

**Type/signature consistency:** `NewScheduledJobsHandler` takes `*ScheduledJobContinueService` as its 3rd arg from Task 14 onward. Tasks 10 and 11 use the 2-arg form — update Task 10's test helper at Task 14 Step 2.

**Known runtime fork:** Task 9 ships `RuntimeAgentRunner` as a stub (`ErrRuntimeNotWired`) because the agent runtime's public surface needs reading before wiring. Step 2 of that task is "read the files first", Step 3 is "replace the stub". The contract (`AgentRunner.RunOnce`) is locked.

**Known cosmetic fork:** Task 13's `strconvItoa` placeholder must become `strconv.Itoa` (with the corresponding import). Called out inline.

---

## Execution Handoff

Plan complete and saved to `.gpowers/plans/2026-05-28-scheduled-jobs.md`. Execution options after Plans C and D are written:

1. **Subagent-Driven** — fresh subagent per task, fast iteration
2. **Inline Execution** — batch with checkpoints

Pick when ready.
