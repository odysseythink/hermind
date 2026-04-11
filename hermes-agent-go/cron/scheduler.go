package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
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
	jobs []Job
	mu   sync.Mutex
}

func NewScheduler() *Scheduler { return &Scheduler{} }

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
			runJobLoop(ctx, j)
		}(j)
	}
	wg.Wait()
	return nil
}

// runJobLoop fires j.Run on each tick.
func runJobLoop(ctx context.Context, j Job) {
	t := time.NewTicker(j.Schedule.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			slog.InfoContext(ctx, "cron: running job", "job", j.Name)
			if err := j.Run(ctx); err != nil {
				slog.ErrorContext(ctx, "cron: job failed", "job", j.Name, "err", err.Error())
			}
		}
	}
}
