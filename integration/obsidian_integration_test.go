package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/obsidian"
)

func TestObsidianToolE2E(t *testing.T) {
	vault := t.TempDir()
	_ = os.WriteFile(filepath.Join(vault, "Hello.md"), []byte("---\ntags:\n  - hello\n---\n\n# Hello\n"), 0o644)

	reg := tool.NewRegistry()
	obsidian.RegisterAll(reg)

	ctx := context.WithValue(context.Background(), obsidian.VaultPathKey, vault)

	// Read note
	result, err := reg.Dispatch(ctx, "obsidian_read_note", []byte(`{"path":"Hello.md"}`))
	if err != nil {
		t.Fatalf("dispatch read: %v", err)
	}
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected Hello in result: %s", result)
	}

	// Search vault
	result, err = reg.Dispatch(ctx, "obsidian_search_vault", []byte(`{"query":"Hello"}`))
	if err != nil {
		t.Fatalf("dispatch search: %v", err)
	}
	if !strings.Contains(result, "Hello.md") {
		t.Errorf("expected Hello.md in search: %s", result)
	}
}
