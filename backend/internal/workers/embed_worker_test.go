package workers

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestEmbedWorkerJob_Enabled(t *testing.T) {
	cfg := &config.Config{}
	job := NewEmbedWorkerJob(nil, cfg, nil, nil)
	assert.False(t, job.Enabled(context.Background()))
}

func TestEmbedWorkerJob_EnqueueAndRun(t *testing.T) {
	cfg := &config.Config{}
	job := NewEmbedWorkerJob(nil, cfg, nil, nil)

	job.Enqueue(EmbedRequest{Files: []string{"a.txt", "b.txt"}, WorkspaceID: 1})

	// Run with nil embedder/vectordb returns error from processRequest
	err := job.Run(context.Background())
	assert.NoError(t, err) // Run itself succeeds even if individual requests error

	// Queue should be drained
	job.mu.Lock()
	assert.Len(t, job.queue, 0)
	job.mu.Unlock()
}
