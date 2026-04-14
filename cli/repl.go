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
	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/cli/ui"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/browser"
	"github.com/odysseythink/hermind/tool/delegate"
	"github.com/odysseythink/hermind/tool/file"
	"github.com/odysseythink/hermind/tool/mcp"
	"github.com/odysseythink/hermind/tool/memory"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/hermind/tool/terminal"
	"github.com/odysseythink/hermind/tool/vision"
	"github.com/odysseythink/hermind/tool/web"
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
			fmt.Fprintf(os.Stderr, "hermind: warning: fallback_providers[%d] (%s) has no api_key — skipping\n", i, fbCfg.Provider)
			continue
		}
		fb, err := factory.New(fbCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hermind: warning: fallback_providers[%d] (%s): %v — skipping\n", i, fbCfg.Provider, err)
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

	// Auxiliary provider for context compression, vision summarization, etc.
	// If not configured, compression is a no-op.
	var auxProvider provider.Provider
	if app.Config.Auxiliary.APIKey != "" || app.Config.Auxiliary.Provider != "" {
		auxCfg := config.ProviderConfig{
			Provider: app.Config.Auxiliary.Provider,
			BaseURL:  app.Config.Auxiliary.BaseURL,
			APIKey:   app.Config.Auxiliary.APIKey,
			Model:    app.Config.Auxiliary.Model,
		}
		if auxCfg.Provider == "" {
			// Default to the same provider as primary
			auxCfg.Provider = "anthropic"
		}
		if auxP, err := factory.New(auxCfg); err == nil {
			auxProvider = auxP
		}
	}

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
		return fmt.Errorf("hermind: create terminal backend %q: %w", app.Config.Terminal.Backend, err)
	}
	defer backend.Close()
	terminal.RegisterShellExecute(toolRegistry, backend)

	// Web tools (always register web_fetch, others if API keys present)
	exaKey := os.Getenv("EXA_API_KEY")
	firecrawlKey := os.Getenv("FIRECRAWL_API_KEY")
	web.RegisterAll(toolRegistry, exaKey, firecrawlKey)

	// Memory tools (require storage)
	if app.Storage != nil {
		memory.RegisterAll(toolRegistry, app.Storage)
	}

	// Browser automation tools (Plan 6d). Registered only when Browserbase
	// credentials are present (via env vars or config).
	browserProvider := browser.NewBrowserbase(app.Config.Browser.Browserbase)
	browser.RegisterAll(toolRegistry, browserProvider)

	// Vision tool (Plan 6e). Uses the auxiliary provider since vision
	// analysis is a secondary-model task. If no aux provider is set,
	// the tool is not registered.
	visionModel := app.Config.Auxiliary.Model
	if visionModel == "" {
		visionModel = displayModel
	}
	vision.Register(toolRegistry, auxProvider, visionModel)

	sessionID := uuid.NewString()

	// External memory provider (Plan 6c / 6c.1 / 6c.2): register the
	// active provider's tools into the main registry and hook Shutdown
	// into the REPL lifecycle.
	extMem, err := memprovider.New(app.Config.Memory, memprovider.WithStorage(app.Storage))
	if err != nil {
		fmt.Fprintf(os.Stderr, "hermind: memory provider: %v\n", err)
	}
	if extMem != nil {
		if err := extMem.Initialize(ctx, sessionID); err != nil {
			fmt.Fprintf(os.Stderr, "hermind: memory provider %s init: %v\n", extMem.Name(), err)
		} else {
			extMem.RegisterTools(toolRegistry)
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = extMem.Shutdown(shutdownCtx)
			}()
		}
	}

	// Delegate tool — the runner spawns a fresh Engine per call
	delegate.RegisterDelegate(toolRegistry, func(ctx context.Context, task, extra string, maxTurns int) (*delegate.SubagentResult, error) {
		// Reuse the same registry; rely on prompt guidance to tell the subagent
		// not to call delegate. Plan 6b can add filter-based registries if
		// recursion becomes a real problem.
		subEngine := agent.NewEngineWithToolsAndAux(
			p, auxProvider, app.Storage, toolRegistry,
			config.AgentConfig{
				MaxTurns:    maxTurns,
				Compression: app.Config.Agent.Compression,
			},
			"subagent",
		)

		result, err := subEngine.RunConversation(ctx, &agent.RunOptions{
			UserMessage: task + "\n\n" + extra,
			SessionID:   sessionID + "-sub",
			Model:       displayModel,
		})
		if err != nil {
			return nil, err
		}
		return &delegate.SubagentResult{
			Response:   result.Response,
			Iterations: result.Iterations,
			ToolCalls:  0, // tracking deferred to Plan 6b
		}, nil
	})

	// MCP: start all configured servers and register their tools.
	var mcpManager *mcp.Manager
	if len(app.Config.MCP.Servers) > 0 {
		mcpManager = mcp.NewManager("0.1.0", toolRegistry)

		var serverCfgs []mcp.ServerConfig
		for name, srv := range app.Config.MCP.Servers {
			if !srv.IsEnabled() {
				continue
			}
			serverCfgs = append(serverCfgs, mcp.ServerConfig{
				Name:    name,
				Command: srv.Command,
				Args:    srv.Args,
				Env:     srv.Env,
			})
		}

		if err := mcpManager.Start(ctx, serverCfgs); err != nil {
			fmt.Fprintf(os.Stderr, "hermind: mcp warning: %v\n", err)
		}
		defer mcpManager.Close()
	}

	// Hand off to the bubbletea TUI.
	err = ui.Run(ctx, ui.RunOptions{
		Config:      app.Config,
		Storage:     app.Storage,
		Provider:    p,
		AuxProvider: auxProvider,
		ToolReg:     toolRegistry,
		AgentCfg:    app.Config.Agent,
		SessionID:   sessionID,
		Model:       displayModel,
	})
	if err != nil {
		return fmt.Errorf("hermind: tui: %w", err)
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
		path = filepath.Join(home, ".hermind", "state.db")
	}

	// Ensure the parent directory exists
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
		return nil, "", fmt.Errorf("hermind: %s provider is not configured. Set api_key in ~/.hermind/config.yaml or ANTHROPIC_API_KEY env var", primaryName)
	}

	// Default the model field from cfg.Model
	if pCfg.Model == "" {
		pCfg.Model = defaultModelFromString(cfg.Model)
	}

	p, err := factory.New(pCfg)
	if err != nil {
		return nil, "", fmt.Errorf("hermind: create provider: %w", err)
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
