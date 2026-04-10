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
	"github.com/nousresearch/hermes-agent/provider/anthropic"
	"github.com/nousresearch/hermes-agent/storage/sqlite"
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

	// Build the Anthropic provider from config
	anthropicCfg, ok := app.Config.Providers["anthropic"]
	if !ok {
		anthropicCfg = config.ProviderConfig{Provider: "anthropic"}
	}

	// Allow ANTHROPIC_API_KEY env var as fallback when config is missing the key
	if anthropicCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			anthropicCfg.APIKey = envKey
		}
	}

	if anthropicCfg.APIKey == "" {
		return fmt.Errorf("hermes: anthropic provider is not configured. Set api_key in ~/.hermes/config.yaml or ANTHROPIC_API_KEY env var")
	}
	if anthropicCfg.Model == "" {
		anthropicCfg.Model = defaultModelFromString(app.Config.Model)
	}

	p, err := anthropic.New(anthropicCfg)
	if err != nil {
		return fmt.Errorf("hermes: create provider: %w", err)
	}

	// Print the banner and context
	sessionID := uuid.NewString()
	fmt.Print(banner)
	fmt.Printf("  %s · session %s\n\n", anthropicCfg.Model, sessionID[:8])

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var history []message.Message
	turnCount := 0
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
		engine := agent.NewEngine(p, app.Storage, app.Config.Agent, "cli")

		// Register streaming callback: print deltas as they arrive
		engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
			if d != nil && d.Content != "" {
				fmt.Print(d.Content)
			}
		})

		fmt.Println() // newline before streaming output

		result, err := engine.RunConversation(ctx, &agent.RunOptions{
			UserMessage: line,
			History:     history,
			SessionID:   sessionID,
			Model:       anthropicCfg.Model,
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
		history = result.Messages
		turnCount++
		totalUsage.InputTokens += result.Usage.InputTokens
		totalUsage.OutputTokens += result.Usage.OutputTokens
	}

	// Session summary
	fmt.Printf("\nSession #%s: %d messages, %d in / %d out tokens · saved to %s\n",
		sessionID[:8], turnCount*2,
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

// defaultModelFromString parses "anthropic/claude-opus-4-6" into just "claude-opus-4-6".
func defaultModelFromString(s string) string {
	if idx := strings.Index(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

