// agent/ensure_session_test.go
package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func newTestStoreForEngine(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	s, err := sqlite.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newEngineWithStorage(t *testing.T, store storage.Storage, platform string) *Engine {
	t.Helper()
	return NewEngineWithToolsAndAux(nil, nil, store, nil, config.AgentConfig{MaxTurns: 3}, platform)
}

func TestEnsureSession_NewRow_ComposesPromptAndTitle(t *testing.T) {
	store := newTestStoreForEngine(t)
	eng := newEngineWithStorage(t, store, "web")

	opts := &RunOptions{
		SessionID:   "s-new-1",
		UserMessage: "Build me a haiku generator",
		Model:       "claude-opus-4-7",
	}

	sess, created, err := eng.ensureSession(context.Background(), opts, "You are helpful.", opts.UserMessage, opts.Model)
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.True(t, created)
	assert.Equal(t, "You are helpful.\n\nBuild me a haiku generator", sess.SystemPrompt)
	assert.Equal(t, "Build me a", sess.Title) // first 10 runes
	assert.Equal(t, "web", sess.Source)
}

func TestEnsureSession_ExistingRow_Unchanged(t *testing.T) {
	store := newTestStoreForEngine(t)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID:           "s-existing",
		Source:       "telegram",
		Model:        "claude-opus-4-7",
		SystemPrompt: "You are helpful.\n\nfirst ever",
		Title:        "first ever",
		StartedAt:    time.Now().UTC(),
	}))
	eng := newEngineWithStorage(t, store, "web")

	sess, created, err := eng.ensureSession(ctx, &RunOptions{
		SessionID:   "s-existing",
		UserMessage: "second attempt", // MUST be ignored
	}, "new default prompt", "second attempt", "")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, "You are helpful.\n\nfirst ever", sess.SystemPrompt)
	assert.Equal(t, "first ever", sess.Title)
}

func TestEnsureSession_EmptyFirstMessage_FallsBackToDefault(t *testing.T) {
	store := newTestStoreForEngine(t)
	eng := newEngineWithStorage(t, store, "web")

	sess, created, err := eng.ensureSession(context.Background(), &RunOptions{
		SessionID: "s-empty", UserMessage: "   ",
	}, "default", "   ", "m")
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "default", sess.SystemPrompt)
	assert.Equal(t, "", sess.Title)
}
