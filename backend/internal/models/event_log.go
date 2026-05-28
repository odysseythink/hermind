package models

import "time"

type EventLog struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Event      string    `json:"event"`
	Metadata   *string   `json:"metadata,omitempty"`
	UserID     *int      `json:"userId,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}
