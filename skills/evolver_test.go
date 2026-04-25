package skills_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	sqlitestore "github.com/odysseythink/hermind/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvolverExtractUsesVerdict(t *testing.T) {
	dir := t.TempDir()
	ev := skills.NewEvolver(nil, dir) // llm nil — legacy path would no-op
	verdict := &agent.Verdict{
		SkillsToExtract: []agent.SkillDraft{
			{
				Name:        "Reroute on rate-limit",
				Description: "Backoff strategy",
				Body:        "## Reroute on rate-limit\n**When to use:** when provider 429s.\n\nDo this.",
			},
		},
	}
	require.NoError(t, ev.Extract(context.Background(), []message.Message{}, verdict))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	body, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(body), "Reroute on rate-limit")
}

func TestEvolverExtractNilVerdictFallsBackToLegacy(t *testing.T) {
	dir := t.TempDir()
	ev := skills.NewEvolver(nil, dir) // nil llm → no-op legacy path
	require.NoError(t, ev.Extract(context.Background(), []message.Message{}, nil))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no skill files when verdict nil and llm nil")
}

func TestEvolverExtractNoLLM(t *testing.T) {
	dir := t.TempDir()
	evolver := skills.NewEvolver(nil, dir)
	turns := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("how do I reset git?")},
		{Role: message.RoleAssistant, Content: message.TextContent("git reset --hard HEAD")},
	}
	if err := evolver.Extract(context.Background(), turns, nil); err != nil {
		t.Fatalf("Extract without LLM: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected no files written without LLM, got %d", len(entries))
	}
}

func TestEvolverSkillDirCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	evolver := skills.NewEvolver(nil, dir)
	_ = evolver.Extract(context.Background(), nil, nil)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("skills dir not created")
	}
}

func TestEvolverExtractRefreshesTracker(t *testing.T) {
	skillDir := t.TempDir()
	store := newSkillsTestStore(t)
	tracker := skills.NewTracker(store, skillDir)

	// Mock LLM that returns a non-empty skill markdown
	mockLLM := &mockProvider{
		name: "test",
		resp: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent("## Test Skill\n\n**When to use:** test\n\nDo this."),
			},
		},
	}

	ev := skills.NewEvolver(mockLLM, skillDir).WithTracker(tracker)

	// Pass a non-empty conversation to trigger the LLM path
	turns := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("hello")},
	}
	require.NoError(t, ev.Extract(context.Background(), turns, nil))

	gen, err := store.GetSkillsGeneration(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), gen.Seq, "Evolver.Extract must trigger Tracker.Refresh on write")
}

func TestEvolverExtractNoWriteNoBump(t *testing.T) {
	skillDir := t.TempDir()
	store := newSkillsTestStore(t)
	tracker := skills.NewTracker(store, skillDir)

	// Mock LLM that returns empty string — no skill to write
	mockLLM := &mockProvider{
		name: "test",
		resp: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent(""),
			},
		},
	}

	ev := skills.NewEvolver(mockLLM, skillDir).WithTracker(tracker)
	turns := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("hello")},
	}
	require.NoError(t, ev.Extract(context.Background(), turns, nil))

	gen, err := store.GetSkillsGeneration(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), gen.Seq, "Evolver.Extract must not trigger Tracker.Refresh on no-write")
}

// mockProvider is a test helper implementing provider.Provider
type mockProvider struct {
	name      string
	err       error
	resp      *provider.Response
	callCount int
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	return nil, errors.New("not implemented")
}
func (m *mockProvider) ModelInfo(string) *provider.ModelInfo                { return nil }
func (m *mockProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (m *mockProvider) Available() bool                            { return true }

// newSkillsTestStore creates a test-scoped storage store.
func newSkillsTestStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	st, err := sqlitestore.Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	require.NoError(t, st.Migrate())
	t.Cleanup(func() { _ = st.Close() })
	return st
}
