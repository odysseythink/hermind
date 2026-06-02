package agent

import (
	agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"gorm.io/gorm"
)

// buildCompressor constructs a ContextEngine for the agent path when compression
// is enabled (globally or per-workspace). It wraps the Pantheon Compressor in an
// Observer that persists extracted summaries to thread_compactions.
func buildCompressor(db *gorm.DB, ws *models.Workspace, lm core.LanguageModel, sysSvc *services.SystemService) agentcompression.ContextEngine {
	store := agentcompression.NewCompactionStore(db)
	comp := agentcompression.NewForAgent(lm, ws, store)
	if comp == nil {
		return nil
	}

	obs := agentcompression.NewObserver(comp, func(summary string) error {
		return store.Save(&models.ThreadCompaction{
			WorkspaceID: ws.ID,
			ThreadID:    nil, // agent sessions have no thread
			Summary:     summary,
		})
	})
	return obs
}
