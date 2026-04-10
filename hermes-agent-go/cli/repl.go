// cli/repl.go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nousresearch/hermes-agent/cli/ui"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/factory"
	"github.com/nousresearch/hermes-agent/storage/sqlite"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/nousresearch/hermes-agent/tool/file"
	"github.com/nousresearch/hermes-agent/tool/terminal"
)

// runREPL starts the interactive TUI session.
// Plan 4: delegates to the bubbletea TUI via ui.Run.
func runREPL(ctx context.Context, app *App) error {
	// Open storage lazily
	if err := ensureStorage(app); err != nil {
		return err
	}

	// Build the primary provider via factory
	primaryProvider, primaryName, err := buildPrimaryProvider(app.Config)
	if err != nil {
		return err
	}
	_ = primaryName

	// Build fallback providers (skip entries with empty api_key)
	providers := []provider.Provider{primaryProvider}
	for i, fbCfg := range app.Config.FallbackProviders {
		if fbCfg.APIKey == "" {
			fmt.Fprintf(os.Stderr, "hermes: warning: fallback_providers[%d] (%s) has no api_key — skipping\n", i, fbCfg.Provider)
			continue
		}
		fb, err := factory.New(fbCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hermes: warning: fallback_providers[%d] (%s): %v — skipping\n", i, fbCfg.Provider, err)
			continue
		}
		providers = append(providers, fb)
	}

	// Use FallbackChain when multiple providers are configured
	var p provider.Provider
	if len(providers) == 1 {
		p = providers[0]
	} else {
		p = provider.NewFallbackChain(providers)
	}

	displayModel := defaultModelFromString(app.Config.Model)

	// Register built-in tools
	toolRegistry := tool.NewRegistry()
	file.RegisterAll(toolRegistry)
	termCfg := terminal.Config{
		Cwd:              app.Config.Terminal.Cwd,
		DockerImage:      app.Config.Terminal.DockerImage,
		DockerVolumes:    app.Config.Terminal.DockerVolumes,
		SSHHost:          app.Config.Terminal.SSHHost,
		SSHUser:          app.Config.Terminal.SSHUser,
		SSHKey:           app.Config.Terminal.SSHKey,
		SingularityImage: app.Config.Terminal.SingularityImage,
		ModalBaseURL:     app.Config.Terminal.ModalBaseURL,
		ModalToken:       app.Config.Terminal.ModalToken,
		DaytonaBaseURL:   app.Config.Terminal.DaytonaBaseURL,
		DaytonaToken:     app.Config.Terminal.DaytonaToken,
	}
	if app.Config.Terminal.Timeout > 0 {
		termCfg.Timeout = time.Duration(app.Config.Terminal.Timeout) * time.Second
	}

	backend, err := terminal.New(app.Config.Terminal.Backend, termCfg)
	if err != nil {
		return fmt.Errorf("hermes: create terminal backend %q: %w", app.Config.Terminal.Backend, err)
	}
	defer backend.Close()
	terminal.RegisterShellExecute(toolRegistry, backend)

	sessionID := uuid.NewString()

	// Hand off to the bubbletea TUI.
	err = ui.Run(ctx, ui.RunOptions{
		Config:    app.Config,
		Storage:   app.Storage,
		Provider:  p,
		ToolReg:   toolRegistry,
		AgentCfg:  app.Config.Agent,
		SessionID: sessionID,
		Model:     displayModel,
	})
	if err != nil {
		return fmt.Errorf("hermes: tui: %w", err)
	}
	return nil
}

// ensureStorage opens the SQLite store on first use and runs migrations.
func ensureStorage(app *App) error {
	if app.Storage != nil {
		return nil
	}
	path := app.Config.Storage.SQLitePath
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".hermes", "state.db")
	}

	// Ensure the parent directory exists
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("hermes: create db dir: %w", err)
		}
	}

	store, err := sqlite.Open(path)
	if err != nil {
		return fmt.Errorf("hermes: open storage: %w", err)
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		return fmt.Errorf("hermes: migrate: %w", err)
	}
	app.Storage = store
	return nil
}

// buildPrimaryProvider constructs the primary provider from the config.
// It parses cfg.Model (e.g., "anthropic/claude-opus-4-6") to determine the
// provider name, looks up the matching entry in cfg.Providers, and calls
// factory.New. For anthropic, it falls back to the ANTHROPIC_API_KEY env var
// if api_key is missing from config.
func buildPrimaryProvider(cfg *config.Config) (provider.Provider, string, error) {
	// Parse "provider/model" to extract the provider name
	primaryName := cfg.Model
	if idx := strings.Index(cfg.Model, "/"); idx >= 0 {
		primaryName = cfg.Model[:idx]
	}

	// Look up provider config (default to empty if missing)
	pCfg, ok := cfg.Providers[primaryName]
	if !ok {
		pCfg = config.ProviderConfig{Provider: primaryName}
	}
	if pCfg.Provider == "" {
		pCfg.Provider = primaryName
	}

	// For anthropic, fall back to ANTHROPIC_API_KEY env var
	if primaryName == "anthropic" && pCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			pCfg.APIKey = envKey
		}
	}

	if pCfg.APIKey == "" {
		return nil, "", fmt.Errorf("hermes: %s provider is not configured. Set api_key in ~/.hermes/config.yaml or ANTHROPIC_API_KEY env var", primaryName)
	}

	// Default the model field from cfg.Model
	if pCfg.Model == "" {
		pCfg.Model = defaultModelFromString(cfg.Model)
	}

	p, err := factory.New(pCfg)
	if err != nil {
		return nil, "", fmt.Errorf("hermes: create provider: %w", err)
	}
	return p, primaryName, nil
}

// defaultModelFromString parses "anthropic/claude-opus-4-6" into just "claude-opus-4-6".
func defaultModelFromString(s string) string {
	if idx := strings.Index(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}
