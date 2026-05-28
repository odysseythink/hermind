package workers

import (
	"context"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

const (
	MinChatsForExtract   = 5
	GroupIdleThresholdMS = 20 * 60 * 1000 // 20 min
)

type ExtractMemoriesJob struct {
	db        *gorm.DB
	memSvc    *services.MemoryService
	extractor *services.MemoryExtractor
	sysSvc    *services.SystemService
}

func NewExtractMemoriesJob(db *gorm.DB, memSvc *services.MemoryService, ext *services.MemoryExtractor, sysSvc *services.SystemService) *ExtractMemoriesJob {
	return &ExtractMemoriesJob{db: db, memSvc: memSvc, extractor: ext, sysSvc: sysSvc}
}

func (j *ExtractMemoriesJob) Name() string     { return "extract-memories" }
func (j *ExtractMemoriesJob) Schedule() string { return "0 */3 * * *" }
func (j *ExtractMemoriesJob) Enabled(ctx context.Context) bool {
	v, _ := j.sysSvc.GetSetting(ctx, "memories_auto_extraction_enabled")
	return v == "true"
}

type groupKey struct{ UserID *int; WorkspaceID int }

func (j *ExtractMemoriesJob) Run(ctx context.Context) error {
	var unprocessed []models.WorkspaceChat
	if err := j.db.WithContext(ctx).
		Where("(memory_processed IS NULL OR memory_processed = ?) AND include = ?", false, true).
		Order("created_at ASC").
		Find(&unprocessed).Error; err != nil {
		return err
	}
	if len(unprocessed) == 0 {
		return nil
	}

	groups := map[groupKey][]models.WorkspaceChat{}
	for _, c := range unprocessed {
		k := groupKey{UserID: c.UserID, WorkspaceID: c.WorkspaceID}
		groups[k] = append(groups[k], c)
	}

	for k, chats := range groups {
		if len(chats) < MinChatsForExtract {
			continue
		}
		// Idle check: skip if last chat younger than threshold.
		if time.Since(chats[len(chats)-1].CreatedAt) < time.Duration(GroupIdleThresholdMS)*time.Millisecond {
			continue
		}
		if err := j.extractor.ProcessGroup(ctx, k.UserID, k.WorkspaceID, chats); err != nil {
			mlog.Warning("extract memories failed",
				mlog.Int("workspace", k.WorkspaceID), mlog.Err(err))
		}
		// Mark processed regardless of extractor outcome — anything-llm behavior.
		ids := make([]int, len(chats))
		for i, c := range chats {
			ids[i] = c.ID
		}
		j.markProcessed(ctx, ids)
	}
	return nil
}

func (j *ExtractMemoriesJob) markProcessed(ctx context.Context, ids []int) {
	if len(ids) == 0 {
		return
	}
	t := true
	if err := j.db.WithContext(ctx).Model(&models.WorkspaceChat{}).
		Where("id IN ?", ids).Update("memory_processed", &t).Error; err != nil {
		mlog.Warning("mark memory_processed failed", mlog.Err(err))
	}
}
