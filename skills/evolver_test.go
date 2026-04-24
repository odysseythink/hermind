package skills_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/skills"
)

func TestEvolverExtractNoLLM(t *testing.T) {
	dir := t.TempDir()
	evolver := skills.NewEvolver(nil, dir)
	turns := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("how do I reset git?")},
		{Role: message.RoleAssistant, Content: message.TextContent("git reset --hard HEAD")},
	}
	if err := evolver.Extract(context.Background(), turns); err != nil {
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
	_ = evolver.Extract(context.Background(), nil)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("skills dir not created")
	}
}
