package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

var generatedFilePattern = regexp.MustCompile(`^[a-z]+-[a-f0-9-]{36}(\.\w+)?$`)

type CleanupGeneratedJob struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewCleanupGeneratedJob(db *gorm.DB, cfg *config.Config) *CleanupGeneratedJob {
	return &CleanupGeneratedJob{DB: db, Cfg: cfg}
}

func (j *CleanupGeneratedJob) Name() string     { return "cleanup-generated-files" }
func (j *CleanupGeneratedJob) Schedule() string { return j.Cfg.WorkerCleanupGeneratedInterval }
func (j *CleanupGeneratedJob) Enabled(ctx context.Context) bool {
	return j.Cfg.WorkerCleanupGeneratedEnabled
}

func (j *CleanupGeneratedJob) Run(ctx context.Context) error {
	outputsDir := filepath.Join(j.Cfg.StorageDir, "outputs")
	entries, err := os.ReadDir(outputsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read outputs dir: %w", err)
	}

	activeRefs, err := j.collectActiveFileRefs(ctx)
	if err != nil {
		return fmt.Errorf("collect active refs: %w", err)
	}

	deleted, failed := 0, 0
	for _, entry := range entries {
		name := entry.Name()
		if !generatedFilePattern.MatchString(name) {
			// Also delete files that don't match naming pattern
		} else if activeRefs[name] {
			continue
		}

		fullPath := filepath.Join(outputsDir, name)
		if err := os.RemoveAll(fullPath); err != nil {
			mlog.Warning("failed to delete generated file", mlog.String("path", fullPath), mlog.Err(err))
			failed++
		} else {
			deleted++
		}
	}

	mlog.Info("cleanup-generated complete", mlog.Int("deleted", deleted), mlog.Int("failed", failed))
	return nil
}

func (j *CleanupGeneratedJob) collectActiveFileRefs(ctx context.Context) (map[string]bool, error) {
	var chats []models.WorkspaceChat
	if err := j.DB.WithContext(ctx).Where("`include` = ?", true).Select("response").Find(&chats).Error; err != nil {
		return nil, err
	}

	refs := make(map[string]bool)
	for _, chat := range chats {
		matches := generatedFilePattern.FindAllString(chat.Response, -1)
		for _, m := range matches {
			refs[m] = true
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(chat.Response), &data); err == nil {
			if files, ok := data["files"].([]interface{}); ok {
				for _, f := range files {
					if s, ok := f.(string); ok {
						refs[s] = true
					}
				}
			}
		}
	}
	return refs, nil
}
