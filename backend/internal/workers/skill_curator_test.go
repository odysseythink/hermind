package workers

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSkillCuratorJob_Run(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AgentSkill{}, &models.SystemSetting{}))

	skillSvc := services.NewAgentSkillService(db)

	ctx := context.Background()

	// Create skills with varying ages
	s1, _ := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "active-skill", Content: "..."})
	s2, _ := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "old-skill", Content: "..."})
	s3, _ := skillSvc.Create(ctx, 1, dto.CreateAgentSkillRequest{Name: "pinned-skill", Content: "..."})

	// Mark pinned
	db.Model(s3).Update("pinned", true)

	// Set s2 created_at to 100 days ago
	db.Model(s2).Update("created_at", db.Raw("datetime('now', '-100 days')"))

	// Run curator with stale=30, archive=90
	counts, err := skillSvc.ApplyCuratorTransitions(ctx, 30, 90)
	require.NoError(t, err)
	assert.Equal(t, 3, counts["checked"])
	assert.Equal(t, 1, counts["archived"])
	assert.Equal(t, 0, counts["marked_stale"])

	// Verify states
	active, _ := skillSvc.GetBySlug(ctx, 1, s1.Slug)
	assert.Equal(t, models.AgentSkillStatusActive, active.Status)

	archived, _ := skillSvc.GetBySlug(ctx, 1, s2.Slug)
	assert.Equal(t, models.AgentSkillStatusArchived, archived.Status)

	pinned, _ := skillSvc.GetBySlug(ctx, 1, s3.Slug)
	assert.Equal(t, models.AgentSkillStatusActive, pinned.Status)
}
