// cli/bootstrap.go
//
// Shared CLI helpers for storage opening, primary-provider construction,
// and the degraded-mode stub provider. Extracted from the deleted
// cli/repl.go and cli/stub_provider.go so non-TUI commands (gateway,
// cron, doctor, web) and the shared BuildEngineDeps helper keep working.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/odysseythink/hermind/storage/sqlite"
)

// errMissingAPIKey is returned by buildPrimaryProvider when the primary
// provider has no API key. runWeb suppresses it so the server still
// boots; the POST /messages handler surfaces 503 to the user.
var errMissingAPIKey = errors.New("hermind: primary provider has no api_key")

// ensureStorage opens the SQLite store on first use and runs migrations.
func ensureStorage(app *App) error {
	if app.Storage != nil {
		return nil
	}
	path := app.Config.Storage.SQLitePath
	if path == "" {
		p, err := config.InstancePath("state.db")
		if err != nil {
			return fmt.Errorf("hermind: resolve instance root: %w", err)
		}
		path = p
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("hermind: create db dir: %w", err)
		}
	}
	store, err := sqlite.Open(path)
	if err != nil {
		return fmt.Errorf("hermind: open storage: %w", err)
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		return fmt.Errorf("hermind: migrate: %w", err)
	}
	app.Storage = store
	return nil
}

// buildPrimaryProvider constructs the primary provider from the config.
// Parses cfg.Model (e.g. "anthropic/claude-opus-4-6") to extract the
// provider name, looks it up in cfg.Providers, and calls factory.New.
// Falls back to ANTHROPIC_API_KEY env for anthropic when config is empty.
func buildPrimaryProvider(cfg *config.Config) (provider.Provider, string, error) {
	primaryName := cfg.Model
	if idx := strings.Index(cfg.Model, "/"); idx >= 0 {
		primaryName = cfg.Model[:idx]
	}

	pCfg, ok := cfg.Providers[primaryName]
	if !ok {
		pCfg = config.ProviderConfig{Provider: primaryName}
	}
	if pCfg.Provider == "" {
		pCfg.Provider = primaryName
	}

	if primaryName == "anthropic" && pCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			pCfg.APIKey = envKey
		}
	}

	if pCfg.APIKey == "" {
		return nil, primaryName, fmt.Errorf("%w: provider %q. Set api_key in <instance>/config.yaml or ANTHROPIC_API_KEY env var", errMissingAPIKey, primaryName)
	}

	if pCfg.Model == "" {
		pCfg.Model = defaultModelFromString(cfg.Model)
	}

	p, err := factory.New(pCfg)
	if err != nil {
		return nil, "", fmt.Errorf("hermind: create provider: %w", err)
	}
	return p, primaryName, nil
}

// defaultModelFromString parses "anthropic/claude-opus-4-6" into just
// "claude-opus-4-6".
func defaultModelFromString(s string) string {
	if idx := strings.Index(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

// stubProvider is a provider.Provider that fails every call with a
// helpful "configure a provider" message. BuildEngineDeps installs it
// when no primary provider is configured so the web server can still
// boot — POST /messages then surfaces a clear 503.
type stubProvider struct{ name string }

func newStubProvider(name string) *stubProvider {
	if name == "" {
		name = "unknown"
	}
	return &stubProvider{name: name}
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return nil, s.notConfiguredErr()
}

func (s *stubProvider) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return nil, s.notConfiguredErr()
}

func (s *stubProvider) ModelInfo(_ string) *provider.ModelInfo { return nil }

func (s *stubProvider) EstimateTokens(_ string, text string) (int, error) {
	return len(text) / 4, nil
}

func (s *stubProvider) Available() bool { return false }

func (s *stubProvider) notConfiguredErr() error {
	return fmt.Errorf("hermind: provider %q not configured — open the web UI Settings panel to add an api_key", s.name)
}
