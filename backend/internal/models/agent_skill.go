package models

import "time"

const (
	AgentSkillStatusActive   = "active"
	AgentSkillStatusStale    = "stale"
	AgentSkillStatusArchived = "archived"
)

const (
	AgentSkillCreatedByAgent = "agent"
	AgentSkillCreatedByUser  = "user"
)

// AgentSkill is the agent's procedural memory — reusable approaches for
// recurring task types. Scoped per workspace.
type AgentSkill struct {
	ID            int        `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID   int        `gorm:"index;not null" json:"workspaceId"`
	Name          string     `gorm:"not null" json:"name"`
	Slug          string     `gorm:"not null" json:"slug"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	Content       string     `gorm:"type:text" json:"content"`     // SKILL.md body after frontmatter
	Frontmatter   string     `gorm:"type:text" json:"frontmatter"` // Raw YAML frontmatter
	Status        string     `gorm:"default:active" json:"status"` // active | stale | archived
	Pinned        bool       `gorm:"default:false" json:"pinned"`
	UseCount      int        `gorm:"default:0" json:"useCount"`
	ViewCount     int        `gorm:"default:0" json:"viewCount"`
	PatchCount    int        `gorm:"default:0" json:"patchCount"`
	LastUsedAt    *time.Time `json:"lastUsedAt"`
	LastViewedAt  *time.Time `json:"lastViewedAt"`
	LastPatchedAt *time.Time `json:"lastPatchedAt"`
	CreatedBy     string     `gorm:"default:agent" json:"createdBy"` // "agent" | "user"
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (AgentSkill) TableName() string { return "agent_skills" }

// AgentSkillFile is a supporting file within a skill directory
// (references/, templates/, scripts/, assets/).
type AgentSkillFile struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SkillID   int       `gorm:"index;not null" json:"skillId"`
	FilePath  string    `gorm:"not null" json:"filePath"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AgentSkillFile) TableName() string { return "agent_skill_files" }
