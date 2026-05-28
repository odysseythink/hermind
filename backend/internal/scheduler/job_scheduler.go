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
	inflightCxl map[int]context.CancelFunc // runID -> cancel
	jobOfRun    map[int]int                // runID -> jobID

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
		inflightCxl: make(map[int]context.CancelFunc),
		jobOfRun:    make(map[int]int),
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
	if err := s.sjSvc.UpdateRunTimestamps(ctx, jobID); err != nil {
		mlog.Warning("scheduler: UpdateRunTimestamps failed", mlog.Int("job", jobID), mlog.Err(err))
	}
	s.wg.Add(1)
	go s.executeRun(jobID, run.ID)
	return run, nil
}

// KillRun marks a run failed and cancels its goroutine.
func (s *JobScheduler) KillRun(ctx context.Context, runID int) (bool, error) {
	_, err := s.sjSvc.GetRun(ctx, runID)
	if err != nil {
		return false, err
	}
	killed, err := s.sjSvc.Kill(ctx, runID)
	if err != nil || !killed {
		return false, err
	}
	s.cancelInflight(runID)
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

func (s *JobScheduler) cancelInflight(runID int) {
	s.mu.Lock()
	fn, ok := s.inflightCxl[runID]
	delete(s.inflightCxl, runID)
	delete(s.jobOfRun, runID)
	s.mu.Unlock()
	if ok {
		fn()
	}
}

func (s *JobScheduler) killAllForJob(jobID int) {
	s.mu.Lock()
	var toCancel []int
	for runID, jid := range s.jobOfRun {
		if jid == jobID {
			toCancel = append(toCancel, runID)
		}
	}
	s.mu.Unlock()
	for _, runID := range toCancel {
		s.cancelInflight(runID)
	}
}

func (s *JobScheduler) executeRun(jobID, runID int) {
	defer s.wg.Done()
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	// Acquire fresh context with timeout + cancel func
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.Timeout)
	defer cancel()
	s.mu.Lock()
	s.inflightCxl[runID] = cancel
	s.jobOfRun[runID] = jobID
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.inflightCxl, runID)
		delete(s.jobOfRun, runID)
		s.mu.Unlock()
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
