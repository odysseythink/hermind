package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadNote(t *testing.T) {
	vault := t.TempDir()
	notePath := filepath.Join(vault, "Projects", "Idea.md")
	_ = os.MkdirAll(filepath.Dir(notePath), 0o755)
	_ = os.WriteFile(notePath, []byte("---\ntags:\n  - idea\n---\n\n# My Idea\n\nContent here."), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	result, err := readNoteHandler(ctx, []byte(`{"path":"Projects/Idea.md"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "My Idea") {
		t.Errorf("expected note content, got: %s", result)
	}
	if !strings.Contains(result, "idea") {
		t.Errorf("expected front-matter tag, got: %s", result)
	}
}
