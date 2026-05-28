package workers

import (
	"context"
	"fmt"
	"sync"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/embedder"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

// EmbedRequest represents a single embedding task.
type EmbedRequest struct {
	Files       []string
	WorkspaceID int
	UserID      *int
}

type EmbedWorkerJob struct {
	DB       *gorm.DB
	Cfg      *config.Config
	Emb      embedder.Embedder
	VectorDB vectordb.VectorDatabase

	mu    sync.Mutex
	queue []EmbedRequest
}

func NewEmbedWorkerJob(db *gorm.DB, cfg *config.Config, emb embedder.Embedder, vectorDB vectordb.VectorDatabase) *EmbedWorkerJob {
	return &EmbedWorkerJob{
		DB:       db,
		Cfg:      cfg,
		Emb:      emb,
		VectorDB: vectorDB,
		queue:    make([]EmbedRequest, 0),
	}
}

func (j *EmbedWorkerJob) Name() string     { return "embed-worker" }
func (j *EmbedWorkerJob) Schedule() string { return "" }
func (j *EmbedWorkerJob) Enabled(ctx context.Context) bool {
	return j.Emb != nil && j.VectorDB != nil
}

// Enqueue adds an embedding request to the queue.
func (j *EmbedWorkerJob) Enqueue(req EmbedRequest) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.queue = append(j.queue, req)
}

func (j *EmbedWorkerJob) Run(ctx context.Context) error {
	j.mu.Lock()
	if len(j.queue) == 0 {
		j.mu.Unlock()
		return nil
	}
	batch := j.queue
	j.queue = make([]EmbedRequest, 0)
	j.mu.Unlock()

	for _, req := range batch {
		if err := j.processRequest(ctx, req); err != nil {
			mlog.Error("embed-worker: request failed", mlog.Err(err))
		}
	}
	return nil
}

func (j *EmbedWorkerJob) processRequest(ctx context.Context, req EmbedRequest) error {
	mlog.Info("embed-worker: processing request", mlog.Int("workspaceId", req.WorkspaceID), mlog.Int("fileCount", len(req.Files)))
	// Full embedding pipeline integration is pending DocumentService support.
	// For now, the queue structure is established.
	return fmt.Errorf("embed-worker: full implementation pending integration with DocumentService")
}
