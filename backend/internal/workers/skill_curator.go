package workers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

// SkillCuratorJob periodically reviews agent-created skills and transitions
// their lifecycle states based on inactivity (active → stale → archived).
type SkillCuratorJob struct {
	db        *gorm.DB
	skillSvc  services.AgentSkillManager
	sysSvc    *services.SystemService
	backupSvc services.BackupManager
}

func NewSkillCuratorJob(db *gorm.DB, skillSvc services.AgentSkillManager, sysSvc *services.SystemService) *SkillCuratorJob {
	return &SkillCuratorJob{db: db, skillSvc: skillSvc, sysSvc: sysSvc, backupSvc: nil}
}

func NewSkillCuratorJobWithBackup(db *gorm.DB, skillSvc services.AgentSkillManager, sysSvc *services.SystemService, backupSvc services.BackupManager) *SkillCuratorJob {
	return &SkillCuratorJob{db: db, skillSvc: skillSvc, sysSvc: sysSvc, backupSvc: backupSvc}
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

	// Backup before curator transitions — iterate all workspaces
	if j.backupSvc != nil {
		var workspaces []models.Workspace
		if err := j.db.WithContext(ctx).Find(&workspaces).Error; err != nil {
			mlog.Error("curator: failed to list workspaces for backup", mlog.Err(err))
			return fmt.Errorf("list workspaces: %w", err)
		}
		for _, ws := range workspaces {
			snapshotID, err := j.backupSvc.Snapshot(ctx, ws.ID)
			if err != nil {
				mlog.Error("curator: backup failed, aborting for workspace", mlog.Int("workspace", ws.ID), mlog.Err(err))
				return fmt.Errorf("backup failed for workspace %d: %w", ws.ID, err)
			}
			mlog.Info("curator: backup complete", mlog.Int("workspace", ws.ID), mlog.String("snapshot", snapshotID))
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
