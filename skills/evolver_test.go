package skills_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	sqlitestore "github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/pantheon/core"
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
	require.NoError(t, ev.Extract(context.Background(), []message.HermindMessage{}, verdict))

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
	require.NoError(t, ev.Extract(context.Background(), []message.HermindMessage{}, nil))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no skill files when verdict nil and llm nil")
}

func TestEvolverExtractNoLLM(t *testing.T) {
	dir := t.TempDir()
	evolver := skills.NewEvolver(nil, dir)
	turns := []message.HermindMessage{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("how do I reset git?")},
		{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("git reset --hard HEAD")},
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
		resp: "## Test Skill\n\n**When to use:** test\n\nDo this.",
	}

	ev := skills.NewEvolver(mockLLM, skillDir).WithTracker(tracker)

	// Pass a non-empty conversation to trigger the LLM path
	turns := []message.HermindMessage{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("hello")},
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
		resp: "",
	}

	ev := skills.NewEvolver(mockLLM, skillDir).WithTracker(tracker)
	turns := []message.HermindMessage{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("hello")},
	}
	require.NoError(t, ev.Extract(context.Background(), turns, nil))

	gen, err := store.GetSkillsGeneration(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), gen.Seq, "Evolver.Extract must not trigger Tracker.Refresh on no-write")
}

// mockProvider is a test helper implementing core.LanguageModel
type mockProvider struct {
	name      string
	err       error
	resp      string
	callCount int
}

func (m *mockProvider) Provider() string { return m.name }
func (m *mockProvider) Model() string    { return "test-model" }
func (m *mockProvider) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return &core.Response{
		Message: core.Message{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: []core.ContentParter{core.TextPart{Text: m.resp}},
		},
	}, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockProvider) GenerateObject(context.Context, *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

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
