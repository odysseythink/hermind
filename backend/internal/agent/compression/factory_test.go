package compression

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewForAgent_Defaults(t *testing.T) {
	// Use a nil DB — the factory doesn't touch the DB, only the store wrapper
	store := NewCompactionStore(nil)
	ws := &models.Workspace{}

	comp := NewForAgent(nil, ws, store) // nil LLM is OK when disabled
	// When global is disabled and workspace has no override, comp should be nil
	assert.Nil(t, comp)
}

func TestNewForAgent_Enabled(t *testing.T) {
	store := NewCompactionStore(nil)
	ws := &models.Workspace{
		ChatModel:       strPtr("gpt-4o"),
		CompressEnabled: boolPtr(true),
	}

	// We pass a nil LLM here — the test only validates config wiring,
	// not actual compression. A real caller passes a built Pantheon model.
	comp := NewForAgent(nil, ws, store)
	require.NotNil(t, comp)
}

func TestNewForChat_EnabledWithOverride(t *testing.T) {
	store := NewCompactionStore(nil)
	ws := &models.Workspace{
		ChatModel:         strPtr("gpt-4o-mini"),
		CompressEnabled:   boolPtr(true),
		CompressThreshold: floatPtr(0.80),
	}

	comp := NewForChat(nil, ws, store)
	require.NotNil(t, comp)
}

func TestIsEnabledForWorkspace_GlobalDisable(t *testing.T) {
	// Global disabled, workspace nil -> false
	assert.False(t, IsEnabledForWorkspace(false, nil))

	// Global disabled, workspace explicitly enabled -> true (workspace wins)
	assert.True(t, IsEnabledForWorkspace(false, boolPtr(true)))
}

func TestIsEnabledForWorkspace_GlobalEnable(t *testing.T) {
	// Global enabled, workspace nil -> true
	assert.True(t, IsEnabledForWorkspace(true, nil))

	// Global enabled, workspace explicitly disabled -> false (workspace wins)
	assert.False(t, IsEnabledForWorkspace(true, boolPtr(false)))
}

func TestContextLengthFor_Integration(t *testing.T) {
	// Verify that the factory would pick up the correct context length
	ws := &models.Workspace{ChatModel: strPtr("claude-3-opus-20240229")}
	ctxLen := ContextLengthFor(ptrStr(ws.ChatModel))
	assert.Equal(t, 200000, ctxLen)
}

func strPtr(s string) *string    { return &s }
func boolPtr(b bool) *bool       { return &b }
func floatPtr(f float64) *float64 { return &f }
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
