// cli/repl.go
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/factory"
	"github.com/nousresearch/hermes-agent/storage/sqlite"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/nousresearch/hermes-agent/tool/file"
	"github.com/nousresearch/hermes-agent/tool/terminal"
)

const banner = `
╭─────────────────────────╮
│    HERMES AGENT         │
╰─────────────────────────╯
`

// runREPL starts the interactive read-eval-print loop.
// For Plan 1 this is a minimal bufio.Scanner-based REPL.
// Plan 4 replaces this with a bubbletea TUI.
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
	_ = primaryName

	// Register built-in tools
	toolRegistry := tool.NewRegistry()
	file.RegisterAll(toolRegistry)
	localBackend, err := terminal.NewLocal(terminal.Config{})
	if err != nil {
		return fmt.Errorf("hermes: create terminal backend: %w", err)
	}
	defer localBackend.Close()
	terminal.RegisterShellExecute(toolRegistry, localBackend)

	// Print the banner and context
	sessionID := uuid.NewString()
	fmt.Print(banner)
	fmt.Printf("  %s · session %s\n\n", displayModel, sessionID[:8])

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var history []message.Message
	turnCount := 0
	toolCallCount := 0
	totalUsage := message.Usage{}

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			// Ctrl+D or EOF
			err := scanner.Err()
			if err != nil && err != io.EOF {
				return err
			}
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			break
		}
		if line == "/help" {
			fmt.Println("Commands: /exit /quit /help")
			continue
		}

		// Build engine fresh per turn (single-use semantics)
		engine := agent.NewEngineWithTools(p, app.Storage, toolRegistry, app.Config.Agent, "cli")

		// Register streaming callback: print deltas as they arrive
		engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
			if d != nil && d.Content != "" {
				fmt.Print(d.Content)
			}
		})
		// Tool start: print the tool call header
		engine.SetToolStartCallback(func(call message.ContentBlock) {
			fmt.Printf("\n⚡ %s: %s\n", call.ToolUseName, string(call.ToolUseInput))
		})
		// Tool result: print a truncated snippet of the result
		engine.SetToolResultCallback(func(call message.ContentBlock, result string) {
			snippet := result
			if len(snippet) > 300 {
				snippet = snippet[:300] + "\n... [truncated]"
			}
			// Indent each line with "│ " and mark the end with "└"
			lines := strings.Split(snippet, "\n")
			for _, line := range lines {
				fmt.Printf("│ %s\n", line)
			}
			fmt.Println("└")
		})

		fmt.Println() // newline before streaming output

		result, err := engine.RunConversation(ctx, &agent.RunOptions{
			UserMessage: line,
			History:     history,
			SessionID:   sessionID,
			Model:       displayModel,
		})
		if err != nil {
			if ctx.Err() != nil {
				fmt.Println("\n[interrupted]")
				break
			}
			fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
			continue
		}

		fmt.Println() // newline after streaming

		// Count tool calls in this turn's messages beyond the existing history
		newMessages := result.Messages[len(history):]
		for _, m := range newMessages {
			if !m.Content.IsText() {
				for _, b := range m.Content.Blocks() {
					if b.Type == "tool_use" {
						toolCallCount++
					}
				}
			}
		}
		history = result.Messages
		turnCount++
		totalUsage.InputTokens += result.Usage.InputTokens
		totalUsage.OutputTokens += result.Usage.OutputTokens
	}

	// Session summary
	fmt.Printf("\nSession #%s: %d messages, %d tool calls, %d in / %d out tokens · saved to %s\n",
		sessionID[:8], turnCount*2, toolCallCount,
		totalUsage.InputTokens, totalUsage.OutputTokens,
		app.Config.Storage.SQLitePath,
	)
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
