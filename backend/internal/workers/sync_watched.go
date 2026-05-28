package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

type SyncWatchedJob struct {
	DB   *gorm.DB
	Cfg  *config.Config
	Coll *collector.Client
}

func NewSyncWatchedJob(db *gorm.DB, cfg *config.Config, coll *collector.Client) *SyncWatchedJob {
	return &SyncWatchedJob{DB: db, Cfg: cfg, Coll: coll}
}

func (j *SyncWatchedJob) Name() string     { return "sync-watched-documents" }
func (j *SyncWatchedJob) Schedule() string { return j.Cfg.WorkerSyncWatchedInterval }
func (j *SyncWatchedJob) Enabled(ctx context.Context) bool {
	if !j.Cfg.WorkerSyncWatchedEnabled {
		return false
	}
	var count int64
	j.DB.WithContext(ctx).Model(&models.DocumentSyncQueue{}).
		Where("next_sync_at <= ?", time.Now()).Count(&count)
	return count > 0
}

func (j *SyncWatchedJob) Run(ctx context.Context) error {
	var queues []models.DocumentSyncQueue
	if err := j.DB.WithContext(ctx).
		Where("next_sync_at <= ?", time.Now()).
		Find(&queues).Error; err != nil {
		return fmt.Errorf("fetch stale queues: %w", err)
	}

	if len(queues) == 0 {
		mlog.Info("sync-watched: no stale documents")
		return nil
	}

	mlog.Info("sync-watched: processing documents", mlog.Int("count", len(queues)))

	for _, q := range queues {
		var doc models.WorkspaceDocument
		if err := j.DB.WithContext(ctx).First(&doc, q.WorkspaceDocID).Error; err != nil {
			mlog.Warning("sync-watched: document not found", mlog.Int("docId", q.WorkspaceDocID))
			continue
		}

		if j.Coll != nil {
			mlog.Info("sync-watched: would re-fetch", mlog.String("doc", doc.Filename))
		}

		now := time.Now()
		q.LastSyncedAt = now
		q.NextSyncAt = now.Add(time.Duration(q.StaleAfterMs) * time.Millisecond)
		if err := j.DB.WithContext(ctx).Save(&q).Error; err != nil {
			mlog.Warning("sync-watched: failed to update queue", mlog.Int("queueId", q.ID), mlog.Err(err))
		}
	}

	return nil
}
