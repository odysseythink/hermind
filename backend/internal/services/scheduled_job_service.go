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
