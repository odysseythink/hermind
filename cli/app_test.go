package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewApp_ConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Pre-create a minimal config so NewApp skips first-run.
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("model: anthropic/claude-opus-4-6\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	defer app.Close()

	if app.ConfigPath != cfgPath {
		t.Errorf("ConfigPath = %q, want %q", app.ConfigPath, cfgPath)
	}
}
