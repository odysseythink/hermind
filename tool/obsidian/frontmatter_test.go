package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFrontMatter(t *testing.T) {
	vault := t.TempDir()
	note := filepath.Join(vault, "Note.md")
	_ = os.WriteFile(note, []byte("---\ntags:\n  - old\n---\n\n# Note\n"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	_, err := updateFrontMatterHandler(ctx, []byte(`{"path":"Note.md","updates":{"tags":["new","tag"]}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(note)
	if !strings.Contains(string(data), "new") {
		t.Errorf("expected updated tag: %s", string(data))
	}
}
