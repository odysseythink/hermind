package models

import "time"

type WorkspaceParsedFile struct {
	ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Filename           string    `gorm:"unique" json:"filename"`
	WorkspaceID        int       `json:"workspaceId"`
	UserID             *int      `json:"userId"`
	ThreadID           *int      `json:"threadId"`
	Metadata           *string   `json:"metadata"`
	TokenCountEstimate *int      `gorm:"default:0" json:"tokenCountEstimate"`
	CreatedAt          time.Time `gorm:"autoCreateTime" json:"createdAt"`
}
