package workers

import (
	"context"
	"strconv"

	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

// SkillCuratorJob periodically reviews agent-created skills and transitions
// their lifecycle states based on inactivity (active → stale → archived).
type SkillCuratorJob struct {
	db       *gorm.DB
	skillSvc services.AgentSkillManager
	sysSvc   *services.SystemService
}

func NewSkillCuratorJob(db *gorm.DB, skillSvc services.AgentSkillManager, sysSvc *services.SystemService) *SkillCuratorJob {
	return &SkillCuratorJob{db: db, skillSvc: skillSvc, sysSvc: sysSvc}
}

func (j *SkillCuratorJob) Name() string     { return "skill-curator" }
func (j *SkillCuratorJob) Schedule() string { return "0 2 * * *" } // Daily at 2 AM

func (j *SkillCuratorJob) Enabled(ctx context.Context) bool {
	v, _ := j.sysSvc.GetSetting(ctx, "agent_skill_curator_enabled")
	return v != "false" // default true
}

func (j *SkillCuratorJob) Run(ctx context.Context) error {
	staleDays := 30
	archiveDays := 90

	if v, err := j.sysSvc.GetSetting(ctx, "agent_skill_stale_after_days"); err == nil && v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			staleDays = d
		}
	}
	if v, err := j.sysSvc.GetSetting(ctx, "agent_skill_archive_after_days"); err == nil && v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			archiveDays = d
		}
	}

	counts, err := j.skillSvc.ApplyCuratorTransitions(ctx, staleDays, archiveDays)
	if err != nil {
		return err
	}

	mlog.Info("skill curator run complete",
		mlog.Int("checked", counts["checked"]),
		mlog.Int("marked_stale", counts["marked_stale"]),
		mlog.Int("archived", counts["archived"]),
		mlog.Int("reactivated", counts["reactivated"]),
	)
	return nil
}
