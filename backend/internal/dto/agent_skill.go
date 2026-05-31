package dto

import "github.com/odysseythink/hermind/backend/internal/models"

type CreateAgentSkillRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
	Content     string `json:"content"`
	Frontmatter string `json:"frontmatter,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`
}

type UpdateAgentSkillRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Content     string `json:"content,omitempty"`
	Frontmatter string `json:"frontmatter,omitempty"`
	Status      string `json:"status,omitempty"`
	Pinned      *bool  `json:"pinned,omitempty"`
}

type PatchAgentSkillRequest struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type PatchSkillFileRequest struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type WriteSkillFileRequest struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type AgentSkillListResponse struct {
	Skills     []models.AgentSkill `json:"skills"`
	Categories []string            `json:"categories"`
	Count      int                 `json:"count"`
}

type AgentSkillFileResponse struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}
