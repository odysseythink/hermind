package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// State represents the manager lifecycle state.
type State int

const (
	StateBooting State = iota
	StateRunning
	StateStopping
	StateStopped
)

type Manager struct {
	cron  *cron.Cron
	jobs  []Job
	db    *gorm.DB
	cfg   *config.Config
	wg    sync.WaitGroup
	mu    sync.RWMutex
	state State
}

func NewManager(db *gorm.DB, cfg *config.Config) *Manager {
	return &Manager{
		cron:  cron.New(),
		jobs:  make([]Job, 0),
		db:    db,
		cfg:   cfg,
		state: StateBooting,
	}
}

func (m *Manager) Register(jobs ...Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, jobs...)
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateBooting {
		return fmt.Errorf("worker manager already started")
	}

	for _, job := range m.jobs {
		schedule := job.Schedule()
		if schedule == "" {
			mlog.Info("worker job registered (on-demand)", mlog.String("job", job.Name()))
			continue
		}
		_, err := m.cron.AddFunc(schedule, m.wrapJob(job))
		if err != nil {
			return fmt.Errorf("register job %q: %w", job.Name(), err)
		}
		mlog.Info("worker job scheduled", mlog.String("job", job.Name()), mlog.String("schedule", schedule))
	}

	m.cron.Start()
	m.state = StateRunning
	mlog.Info("worker manager started", mlog.Int("jobs", len(m.jobs)))
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if m.state != StateRunning {
		m.mu.Unlock()
		return nil
	}
	m.state = StateStopping
	m.mu.Unlock()

	mlog.Info("worker manager stopping")

	// Stop cron scheduler (blocks until running cron jobs finish)
	cronCtx := m.cron.Stop()

	// Also wait for any triggered on-demand jobs
	done := make(chan struct{})
	go func() {
		<-cronCtx.Done()
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mlog.Info("worker manager stopped cleanly")
	case <-ctx.Done():
		mlog.Warning("worker manager stop timed out")
	}

	m.mu.Lock()
	m.state = StateStopped
	m.mu.Unlock()
	return nil
}

func (m *Manager) Trigger(name string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state != StateRunning {
		return fmt.Errorf("worker manager not running")
	}

	for _, job := range m.jobs {
		if job.Name() == name {
			go m.wrapJob(job)()
			return nil
		}
	}
	return fmt.Errorf("unknown job: %s", name)
}

func (m *Manager) wrapJob(job Job) func() {
	return func() {
		if !job.Enabled(context.Background()) {
			return
		}

		m.wg.Add(1)
		defer m.wg.Done()

		start := time.Now()
		name := job.Name()
		mlog.Info("worker job started", mlog.String("job", name))

		defer func() {
			if r := recover(); r != nil {
				mlog.Error("worker job panicked", mlog.String("job", name), mlog.Any("recover", r))
			}
		}()

		// Parse per-job timeout from config (default 5m)
		timeout := 5 * time.Minute
		switch name {
		case "cleanup-orphan-documents":
			if d, err := time.ParseDuration(m.cfg.WorkerCleanupOrphanTimeout); err == nil {
				timeout = d
			}
		case "cleanup-generated-files":
			if d, err := time.ParseDuration(m.cfg.WorkerCleanupGeneratedTimeout); err == nil {
				timeout = d
			}
		case "sync-watched-documents":
			if d, err := time.ParseDuration(m.cfg.WorkerSyncWatchedTimeout); err == nil {
				timeout = d
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := job.Run(ctx); err != nil {
			mlog.Error("worker job failed", mlog.String("job", name), mlog.Err(err))
		} else {
			mlog.Info("worker job finished", mlog.String("job", name), mlog.Duration("duration", time.Since(start)))
		}
	}
}
