package sessionrun

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
	"github.com/odysseythink/hermind/tool"
)

// newTestStoreForRunner opens a fresh sqlite store in a temp dir and
// applies migrations. Caller gets a t.Cleanup registered Close.
func newTestStoreForRunner(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	s, err := sqlite.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// eventsOfType extracts the Events from fakeHub whose Type matches the
// requested value. fakeHub itself is defined in runner_test.go.
func eventsOfType(h *fakeHub, t string) []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []Event
	for _, e := range h.events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func TestRun_PublishesSessionCreated_OnBrandNewSession(t *testing.T) {
	hub := &fakeHub{}
	store := newTestStoreForRunner(t)
	deps := Deps{
		Provider: streamingProvider("ok"),
		Storage:  store,
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 3},
		Hub:      hub,
	}

	err := Run(context.Background(), deps, Request{
		SessionID:   "brand-new-sess",
		UserMessage: "Build me a haiku generator",
	})
	require.NoError(t, err)

	created := eventsOfType(hub, "session_created")
	require.Len(t, created, 1, "expected exactly one session_created event")
	assert.Equal(t, "brand-new-sess", created[0].SessionID)

	dto, ok := created[0].Data.(map[string]any)
	require.True(t, ok, "Data must be map[string]any")
	assert.Equal(t, "brand-new-sess", dto["id"])
	assert.Equal(t, "Build me a", dto["title"]) // 10 runes of "Build me a haiku generator"
	assert.Equal(t, "web", dto["source"])
	// model is no longer passed via Request; the engine falls back to "claude-opus-4-6"
	assert.Equal(t, "claude-opus-4-6", dto["model"])
	// started_at is a non-zero float64 (time in unix seconds, sub-second precision)
	startedAt, ok := dto["started_at"].(float64)
	require.True(t, ok, "started_at must be float64, got %T", dto["started_at"])
	assert.Greater(t, startedAt, 0.0)
	// ended_at is always float64(0) for a brand-new session
	endedAt, ok := dto["ended_at"].(float64)
	require.True(t, ok, "ended_at must be float64, got %T", dto["ended_at"])
	assert.Equal(t, 0.0, endedAt)
}

func TestRun_NoSessionCreated_OnExistingSession(t *testing.T) {
	hub := &fakeHub{}
	store := newTestStoreForRunner(t)
	require.NoError(t, store.CreateSession(context.Background(), &storage.Session{
		ID:           "already-there",
		Source:       "web",
		Model:        "stub",
		SystemPrompt: "p",
		Title:        "t",
		StartedAt:    time.Now().UTC(),
	}))

	deps := Deps{
		Provider: streamingProvider("ok"),
		Storage:  store,
		ToolReg:  tool.NewRegistry(),
		AgentCfg: config.AgentConfig{MaxTurns: 3},
		Hub:      hub,
	}

	err := Run(context.Background(), deps, Request{
		SessionID:   "already-there",
		UserMessage: "hi again",
	})
	require.NoError(t, err)

	assert.Empty(t, eventsOfType(hub, "session_created"))
}
