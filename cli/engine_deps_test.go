package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestAttachSkillsTrackerBumpsOnFirstRefresh(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "a.md"), []byte("hi"), 0o644))

	store, err := sqlite.Open(filepath.Join(tmp, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	tracker := attachSkillsTracker(context.Background(), store, skillDir)
	require.NotNil(t, tracker)

	gen, err := store.GetSkillsGeneration(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), gen.Seq, "attachSkillsTracker must Refresh once at construction")
}

func TestAttachSkillsTrackerNilStoreReturnsNil(t *testing.T) {
	tracker := attachSkillsTracker(context.Background(), nil, "")
	require.Nil(t, tracker)
}

// TestBuildEngineDeps_AuxFallsBackToMainProvider verifies that when no
// auxiliary provider is configured, AuxProvider is populated with the main
// provider. The descriptor advertises this contract; without it the agent
// engine's compressor never instantiates and history compression silently
// no-ops, so long Telegram/web sessions blow past the model context.
func TestBuildEngineDeps_AuxFallsBackToMainProvider(t *testing.T) {
	tmp := t.TempDir()
	store, err := sqlite.Open(filepath.Join(tmp, "h.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	t.Setenv("HOME", tmp)
	t.Setenv("ANTHROPIC_API_KEY", "")

	app := &App{
		Config: &config.Config{
			Model: "anthropic/claude-opus-4-7",
			Providers: map[string]config.ProviderConfig{
				"anthropic": {Provider: "anthropic", APIKey: "sk-test", Model: "claude-opus-4-7"},
			},
			// Auxiliary intentionally left blank.
			Terminal: config.TerminalConfig{Backend: "local"},
			Storage:  config.StorageConfig{Driver: "sqlite"},
		},
		Storage: store,
	}

	deps, cleanup, err := BuildEngineDeps(context.Background(), app)
	if cleanup != nil {
		defer cleanup()
	}
	require.NoError(t, err)
	require.NotNil(t, deps.Provider, "main provider must be set")
	require.NotNil(t, deps.AuxProvider, "AuxProvider must fall back to main provider when auxiliary config is blank")
	require.Same(t, deps.Provider, deps.AuxProvider, "blank-auxiliary fallback should reuse the main provider instance")
}
