package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteNote(t *testing.T) {
	vault := t.TempDir()
	ctx := context.WithValue(context.Background(), VaultPathKey, vault)

	_, err := writeNoteHandler(ctx, []byte(`{"path":"New.md","content":"# New Note","frontmatter":{"tags":["new"]}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(vault, "New.md"))
	content := string(data)
	if !strings.Contains(content, "# New Note") {
		t.Errorf("missing body: %s", content)
	}
	if !strings.Contains(content, "tags:") {
		t.Errorf("missing front-matter: %s", content)
	}
}
