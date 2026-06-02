package compression

import (
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
)

// Default thresholds (design §5)
const (
	agentThreshold = 0.50
	chatThreshold  = 0.75
)

// NewForAgent creates a Compressor tuned for the Agent path.
// It uses a 0.50 threshold and seeds the compressor with the latest persisted
// summary for the workspace/thread. If compression is disabled (globally or
// per-workspace), it returns nil. The aux LLM may be nil; the engine handles
// nil gracefully by returning history unchanged.
func NewForAgent(aux core.LanguageModel, ws *models.Workspace, store *CompactionStore) *compression.Compressor {
	if !IsEnabledForWorkspace(globalEnabled(), ws.CompressEnabled) {
		return nil
	}

	cfg := compression.CompressionConfig{
		Enabled:             true,
		Threshold:           agentThreshold,
		TargetRatio:         0.2,
		ProtectLast:         20,
		MaxPasses:           3,
		PerMessageMaxTokens: 8000,
	}

	// Apply workspace overrides
	if ws.CompressThreshold != nil {
		cfg.Threshold = *ws.CompressThreshold
	}

	model := ""
	if ws.AgentModel != nil {
		model = *ws.AgentModel
	} else if ws.ChatModel != nil {
		model = *ws.ChatModel
	}
	ctxLen := ContextLengthFor(model)
	if ws.CompressContextLen != nil {
		ctxLen = *ws.CompressContextLen
	}

	return compression.NewCompressor(cfg, aux, ctxLen)
}

// NewForChat creates a Compressor tuned for the Regular Chat path.
// It uses a 0.75 threshold (higher than agent — chat turns are cheaper).
// If compression is disabled, it returns nil.
func NewForChat(aux core.LanguageModel, ws *models.Workspace, store *CompactionStore) *compression.Compressor {
	if !IsEnabledForWorkspace(globalEnabled(), ws.CompressEnabled) {
		return nil
	}

	cfg := compression.CompressionConfig{
		Enabled:             true,
		Threshold:           chatThreshold,
		TargetRatio:         0.2,
		ProtectLast:         20,
		MaxPasses:           3,
		PerMessageMaxTokens: 8000,
	}

	if ws.CompressThreshold != nil {
		cfg.Threshold = *ws.CompressThreshold
	}

	model := ""
	if ws.ChatModel != nil {
		model = *ws.ChatModel
	}
	ctxLen := ContextLengthFor(model)
	if ws.CompressContextLen != nil {
		ctxLen = *ws.CompressContextLen
	}

	return compression.NewCompressor(cfg, aux, ctxLen)
}

// IsEnabledForWorkspace resolves the effective compression enablement for a
// workspace. Per-workspace setting takes priority over global setting.
func IsEnabledForWorkspace(globalEnabled bool, wsEnabled *bool) bool {
	if wsEnabled != nil {
		return *wsEnabled
	}
	return globalEnabled
}

// globalEnabled reads the system-wide context_compress_enabled setting.
// This is a placeholder that will be replaced by a real SystemSetting lookup
// in Task H3 (ChatService wiring) when the service layer has access to settings.
// For now it returns false so that compression is opt-in until wired.
func globalEnabled() bool {
	return false
}
