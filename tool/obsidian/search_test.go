package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchVault(t *testing.T) {
	vault := t.TempDir()
	_ = os.WriteFile(filepath.Join(vault, "A.md"), []byte("hello world from A"), 0o644)
	_ = os.WriteFile(filepath.Join(vault, "B.md"), []byte("nothing here"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	result, err := searchVaultHandler(ctx, []byte(`{"query":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "A.md") {
		t.Errorf("expected A.md in results: %s", result)
	}
	if strings.Contains(result, "B.md") {
		t.Errorf("did not expect B.md in results: %s", result)
	}
}
