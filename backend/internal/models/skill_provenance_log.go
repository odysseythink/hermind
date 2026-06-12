package models

import "time"

// SkillProvenanceLog records a full-content snapshot every time a skill
// is created, updated, patched, deleted, or a skill file is written/removed.
type SkillProvenanceLog struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SkillID     int       `gorm:"index;not null" json:"skillId"`
	WorkspaceID int       `gorm:"index;not null" json:"workspaceId"`
	Action      string    `gorm:"not null" json:"action"`      // create | edit | patch | delete | write_file | remove_file
	WriteOrigin string    `gorm:"not null" json:"writeOrigin"` // foreground | background_review | curator
	ActorType   string    `gorm:"not null" json:"actorType"`   // agent | user | system
	ActorID     string    `json:"actorId"`                     // user ID or empty string
	Content     string    `gorm:"type:text" json:"content"`    // full skill body snapshot
	FilePath    string    `json:"filePath"`                    // for file ops, empty for skill body
	CreatedAt   time.Time `json:"createdAt"`
}

func (SkillProvenanceLog) TableName() string { return "skill_provenance_logs" }
