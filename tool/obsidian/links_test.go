package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListLinks(t *testing.T) {
	vault := t.TempDir()
	_ = os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]] and [[C|alias]]"), 0o644)
	_ = os.WriteFile(filepath.Join(vault, "B.md"), []byte("links to [[A]]"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	result, err := listLinksHandler(ctx, []byte(`{"path":"A.md","direction":"both"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "B") {
		t.Errorf("expected outgoing link B: %s", result)
	}
	if !strings.Contains(result, "B.md") {
		t.Errorf("expected incoming link from B: %s", result)
	}
}
