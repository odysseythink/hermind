package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendToNote(t *testing.T) {
	vault := t.TempDir()
	note := filepath.Join(vault, "Note.md")
	_ = os.WriteFile(note, []byte("# Note\n"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	_, err := appendToNoteHandler(ctx, []byte(`{"path":"Note.md","content":"\nAppended."}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(note)
	if !strings.Contains(string(data), "Appended.") {
		t.Errorf("expected appended text: %s", string(data))
	}
}
