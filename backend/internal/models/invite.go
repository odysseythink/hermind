package models

import "time"

type Invite struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Code          string    `gorm:"unique" json:"code"`
	Status        string    `gorm:"default:pending" json:"status"`
	ClaimedBy     *int      `json:"claimedBy"`
	WorkspaceIds  *string   `json:"workspaceIds"`
	CreatedAt     time.Time `json:"createdAt"`
	CreatedBy     int       `json:"createdBy"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}
