package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage/sqlite"
)

// TestBuildEngineDeps_Smoke constructs Deps with a minimal config and
// verifies the essential fields are populated.
func TestBuildEngineDeps_Smoke(t *testing.T) {
	tmp := t.TempDir()
	store, err := sqlite.Open(filepath.Join(tmp, "h.db"))
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Setenv("HOME", tmp)
	// Avoid depending on real provider credentials — missing keys
	// force the stub provider fallback path via errMissingAPIKey.
	t.Setenv("ANTHROPIC_API_KEY", "")

	app := &App{
		Config: &config.Config{
			Model: "anthropic/claude-opus-4-7",
			Providers: map[string]config.ProviderConfig{
				"anthropic": {Provider: "anthropic", APIKey: "sk-test", Model: "claude-opus-4-7"},
			},
			Terminal: config.TerminalConfig{Backend: "local"},
			Storage:  config.StorageConfig{Driver: "sqlite"},
		},
		Storage: store,
	}

	deps, cleanup, err := BuildEngineDeps(context.Background(), app)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("BuildEngineDeps: %v", err)
	}
	if deps.Provider == nil {
		t.Error("Provider nil")
	}
	if deps.Storage == nil {
		t.Error("Storage nil")
	}
	if deps.ToolReg == nil {
		t.Error("ToolReg nil")
	}
}
