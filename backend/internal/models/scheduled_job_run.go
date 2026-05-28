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
