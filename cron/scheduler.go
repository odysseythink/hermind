package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/odysseythink/hermind/metrics"
)

// Job is a single scheduled work unit.
type Job struct {
	Name     string
	Schedule Schedule
	// Run is called on each tick; it should be fast to start and
	// honor ctx cancellation. Scheduler logs any returned error.
	Run func(ctx context.Context) error
}

// Scheduler runs a fixed set of Jobs concurrently.
type Scheduler struct {
	jobs    []Job
	mu      sync.Mutex
	runs    *metrics.Counter
	errors  *metrics.Counter
	history HistoryStore
}

func NewScheduler() *Scheduler { return &Scheduler{} }

// SetHistory attaches a HistoryStore so every job run is persisted.
func (s *Scheduler) SetHistory(h HistoryStore) {
	s.history = h
}

// SetMetrics attaches a metrics registry and registers the standard
// cron metrics into it. Safe to call at most once.
func (s *Scheduler) SetMetrics(reg *metrics.Registry) {
	if reg == nil || s.runs != nil {
		return
	}
	s.runs = reg.NewCounter("cron_job_runs_total", "Total cron job runs.")
	s.errors = reg.NewCounter("cron_job_errors_total", "Total cron job errors.")
}

// Add registers a job. Must be called before Start.
func (s *Scheduler) Add(j Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, j)
}

// Start runs all registered jobs until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	jobs := append([]Job{}, s.jobs...)
	s.mu.Unlock()
	if len(jobs) == 0 {
		return fmt.Errorf("cron: no jobs registered")
	}
	slog.InfoContext(ctx, "cron: scheduler started", "jobs", len(jobs))

	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			s.runJobLoop(ctx, j)
		}(j)
	}
	wg.Wait()
	return nil
}

// runJobLoop fires j.Run on each tick.
func (s *Scheduler) runJobLoop(ctx context.Context, j Job) {
	t := time.NewTicker(j.Schedule.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			slog.InfoContext(ctx, "cron: running job", "job", j.Name)
			if s.runs != nil {
				s.runs.With(map[string]string{"job": j.Name}).Inc()
			}
			start := time.Now().UTC()
			err := j.Run(ctx)
			end := time.Now().UTC()
			status := "ok"
			errMsg := ""
			if err != nil {
				status = "error"
				errMsg = err.Error()
				slog.ErrorContext(ctx, "cron: job failed", "job", j.Name, "err", errMsg)
				if s.errors != nil {
					s.errors.With(map[string]string{"job": j.Name}).Inc()
				}
			}
			if s.history != nil {
				_, _ = s.history.Record(ctx, Run{
					JobName:    j.Name,
					StartedAt:  start,
					EndedAt:    end,
					Status:     status,
					Error:      errMsg,
					DurationMS: end.Sub(start).Milliseconds(),
				})
			}
		}
	}
}
