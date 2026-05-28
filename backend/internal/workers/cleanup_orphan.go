package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

type CleanupOrphanJob struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewCleanupOrphanJob(db *gorm.DB, cfg *config.Config) *CleanupOrphanJob {
	return &CleanupOrphanJob{DB: db, Cfg: cfg}
}

func (j *CleanupOrphanJob) Name() string     { return "cleanup-orphan-documents" }
func (j *CleanupOrphanJob) Schedule() string { return j.Cfg.WorkerCleanupOrphanInterval }
func (j *CleanupOrphanJob) Enabled(ctx context.Context) bool {
	return j.Cfg.WorkerCleanupOrphanEnabled
}

func (j *CleanupOrphanJob) Run(ctx context.Context) error {
	// Fetch all referenced filenames from DB
	var files []models.WorkspaceParsedFile
	if err := j.DB.WithContext(ctx).Select("filename").Find(&files).Error; err != nil {
		return fmt.Errorf("fetch parsed files: %w", err)
	}

	referenced := make(map[string]struct{}, len(files))
	for _, f := range files {
		referenced[f.Filename] = struct{}{}
	}

	storageDir := j.Cfg.StorageDir
	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return fmt.Errorf("read storage dir: %w", err)
	}

	deleted, failed := 0, 0
	for _, entry := range entries {
		if _, ok := referenced[entry.Name()]; ok {
			continue
		}

		fullPath := filepath.Join(storageDir, entry.Name())
		if err := os.RemoveAll(fullPath); err != nil {
			mlog.Warning("failed to delete orphan file", mlog.String("path", fullPath), mlog.Err(err))
			failed++
		} else {
			deleted++
		}
	}

	mlog.Info("cleanup-orphan complete", mlog.Int("deleted", deleted), mlog.Int("failed", failed))
	return nil
}
