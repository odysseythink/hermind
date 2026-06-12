package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

// ProvenanceRecorder records skill mutations for audit trail.
type ProvenanceRecorder interface {
	Record(ctx context.Context, skill *models.AgentSkill, action, filePath, actorType, actorID string) error
}

type ProvenanceService struct {
	db *gorm.DB
}

func NewProvenanceService(db *gorm.DB) *ProvenanceService {
	return &ProvenanceService{db: db}
}

func (s *ProvenanceService) Record(ctx context.Context, skill *models.AgentSkill, action, filePath, actorType, actorID string) error {
	log := models.SkillProvenanceLog{
		SkillID:     skill.ID,
		WorkspaceID: skill.WorkspaceID,
		Action:      action,
		WriteOrigin: skill.WriteOrigin,
		ActorType:   actorType,
		ActorID:     actorID,
		Content:     skill.Content,
		FilePath:    filePath,
	}
	return s.db.WithContext(ctx).Create(&log).Error
}
