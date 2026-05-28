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
