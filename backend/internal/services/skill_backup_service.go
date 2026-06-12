package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

var ErrSnapshotNotFound = errors.New("snapshot not found")

type SnapshotInfo struct {
	SnapshotID  string    `json:"snapshotId"`
	WorkspaceID int       `json:"workspaceId"`
	CreatedAt   time.Time `json:"createdAt"`
	SkillCount  int       `json:"skillCount"`
	FileCount   int       `json:"fileCount"`
}

type BackupManager interface {
	Snapshot(ctx context.Context, workspaceID int) (string, error)
	Restore(ctx context.Context, workspaceID int, snapshotID string) error
	List(ctx context.Context, workspaceID int) ([]SnapshotInfo, error)
	Prune(ctx context.Context, workspaceID int, keep int) error
}

type BackupService struct {
	db       *gorm.DB
	baseDir  string // <StorageDir>/skill-backups
	skillSvc *AgentSkillService
}

func NewBackupService(db *gorm.DB, storageDir string, skillSvc *AgentSkillService) *BackupService {
	return &BackupService{
		db:       db,
		baseDir:  filepath.Join(storageDir, "skill-backups"),
		skillSvc: skillSvc,
	}
}

type snapshotData struct {
	WorkspaceID int                  `json:"workspaceId"`
	Timestamp   time.Time            `json:"timestamp"`
	Skills      []skillSnapshotEntry `json:"skills"`
}

type skillSnapshotEntry struct {
	models.AgentSkill
	Files []models.AgentSkillFile `json:"files"`
}

func (s *BackupService) Snapshot(ctx context.Context, workspaceID int) (string, error) {
	skills, err := s.skillSvc.List(ctx, workspaceID, true)
	if err != nil {
		return "", fmt.Errorf("list skills: %w", err)
	}

	entries := make([]skillSnapshotEntry, 0, len(skills))
	for _, sk := range skills {
		files, err := s.skillSvc.ListFiles(ctx, sk.ID)
		if err != nil {
			return "", fmt.Errorf("list files for skill %d: %w", sk.ID, err)
		}
		entries = append(entries, skillSnapshotEntry{
			AgentSkill: sk,
			Files:      files,
		})
	}

	now := time.Now().UTC()
	snap := snapshotData{
		WorkspaceID: workspaceID,
		Timestamp:   now,
		Skills:      entries,
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}

	dir := filepath.Join(s.baseDir, strconv.Itoa(workspaceID))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	filename := now.Format("20060102-150405") + ".json"
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0640); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	snapshotID := filename[:len(filename)-5]
	_ = s.Prune(ctx, workspaceID, 10)
	return snapshotID, nil
}

func (s *BackupService) Restore(ctx context.Context, workspaceID int, snapshotID string) error {
	path := filepath.Join(s.baseDir, strconv.Itoa(workspaceID), snapshotID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrSnapshotNotFound, snapshotID)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	var snap snapshotData
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var skillIDs []int
		if err := tx.Model(&models.AgentSkill{}).Where("workspace_id = ?", workspaceID).Pluck("id", &skillIDs).Error; err != nil {
			return err
		}
		if len(skillIDs) > 0 {
			if err := tx.Where("skill_id IN ?", skillIDs).Delete(&models.AgentSkillFile{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&models.AgentSkill{}).Error; err != nil {
			return err
		}
		for _, entry := range snap.Skills {
			entry.AgentSkill.ID = 0
			entry.AgentSkill.WorkspaceID = workspaceID
			if err := tx.Create(&entry.AgentSkill).Error; err != nil {
				return err
			}
			for _, f := range entry.Files {
				f.ID = 0
				f.SkillID = entry.AgentSkill.ID
				if err := tx.Create(&f).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *BackupService) List(ctx context.Context, workspaceID int) ([]SnapshotInfo, error) {
	dir := filepath.Join(s.baseDir, strconv.Itoa(workspaceID))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var infos []SnapshotInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()
		snapshotID := name[:len(name)-5]
		info, err := e.Info()
		if err != nil {
			continue
		}

		skillCount, fileCount := 0, 0
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			var snap snapshotData
			if err := json.Unmarshal(data, &snap); err == nil {
				skillCount = len(snap.Skills)
				for _, entry := range snap.Skills {
					fileCount += len(entry.Files)
				}
			}
		}

		infos = append(infos, SnapshotInfo{
			SnapshotID:  snapshotID,
			WorkspaceID: workspaceID,
			CreatedAt:   info.ModTime(),
			SkillCount:  skillCount,
			FileCount:   fileCount,
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].SnapshotID < infos[j].SnapshotID
	})
	return infos, nil
}

func (s *BackupService) Prune(ctx context.Context, workspaceID int, keep int) error {
	infos, err := s.List(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(infos) <= keep {
		return nil
	}
	toDelete := len(infos) - keep
	dir := filepath.Join(s.baseDir, strconv.Itoa(workspaceID))
	for i := 0; i < toDelete; i++ {
		path := filepath.Join(dir, infos[i].SnapshotID+".json")
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("prune %s: %w", infos[i].SnapshotID, err)
		}
	}
	return nil
}
