package skills_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/skills"
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
