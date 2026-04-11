package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newSetupCmd creates the "hermes setup" wizard.
func newSetupCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive configuration wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupInteractive(bufio.NewReader(os.Stdin))
		},
	}
}

// runSetupInteractive walks the user through minimum config and
// writes ~/.hermes/config.yaml.
func runSetupInteractive(reader *bufio.Reader) error {
	fmt.Println("hermes setup — interactive configuration wizard")
	fmt.Println("This writes ~/.hermes/config.yaml. Press Enter to accept defaults.")
	fmt.Println()

	provider := prompt(reader, "Primary provider [anthropic]", "anthropic")
	apiKey := prompt(reader, fmt.Sprintf("%s api key (leave blank to use env var)", provider), "")
	model := prompt(reader, "Default model [claude-opus-4-6]", "claude-opus-4-6")
	terminalBackend := prompt(reader, "Terminal backend [local]", "local")
	storagePath := prompt(reader, "SQLite path [~/.hermes/state.db]", "~/.hermes/state.db")

	cfg := config.Default()
	cfg.Model = provider + "/" + model
	cfg.Providers = map[string]config.ProviderConfig{
		provider: {
			Provider: provider,
			APIKey:   apiKey,
			Model:    model,
		},
	}
	cfg.Terminal.Backend = terminalBackend
	cfg.Storage.SQLitePath = storagePath

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfgDir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	buf, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, buf, 0o644); err != nil {
		return err
	}
	fmt.Printf("\nwrote %s\n", cfgPath)
	fmt.Println("run `hermes doctor` to verify your setup.")
	return nil
}

func prompt(reader *bufio.Reader, label, def string) string {
	fmt.Printf("%s: ", label)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}
