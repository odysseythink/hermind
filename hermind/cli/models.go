package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// knownModels is a small built-in catalog for `hermes models list`.
// It is intentionally static — the full `models_dev.py` catalog from
// the Python reference is out of scope for Phase 11b.
var knownModels = map[string][]string{
	"anthropic":  {"claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5"},
	"openai":     {"gpt-4o", "gpt-4o-mini", "gpt-4.1", "o1", "o3-mini"},
	"openrouter": {"openai/gpt-4o", "anthropic/claude-opus-4-6", "google/gemini-2.5-pro"},
	"deepseek":   {"deepseek-chat", "deepseek-reasoner"},
	"kimi":       {"moonshot-v1-128k"},
	"minimax":    {"MiniMax-M2.7"},
	"qwen":       {"qwen-max", "qwen-plus", "qwen-turbo"},
	"wenxin":     {"ernie-4.0", "ernie-3.5"},
	"zhipu":      {"glm-4.5", "glm-4.5-air"},
}

func newModelsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage the active model and inspect the catalog",
	}
	cmd.AddCommand(newModelsListCmd())
	cmd.AddCommand(newModelsSwitchCmd())
	cmd.AddCommand(newModelsCurrentCmd(app))
	return cmd
}

func newModelsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [provider]",
		Short: "List known models (optionally filter by provider)",
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}
			for provider, models := range knownModels {
				if filter != "" && filter != provider {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", provider)
				for _, m := range models {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s/%s\n", provider, m)
				}
			}
			return nil
		},
	}
}

func newModelsSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <provider/model>",
		Short: "Switch the active model in ~/.hermind/config.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			cfgPath := filepath.Join(home, ".hermind", "config.yaml")
			data, err := os.ReadFile(cfgPath)
			if err != nil {
				return fmt.Errorf("read %s: %w", cfgPath, err)
			}
			// Parse as a generic map so we can surgically update
			// the model field without touching anything else.
			var m map[string]any
			if err := yaml.Unmarshal(data, &m); err != nil {
				return err
			}
			m["model"] = args[0]
			out, err := yaml.Marshal(m)
			if err != nil {
				return err
			}
			if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "active model set to %s\n", args[0])
			return nil
		},
	}
}

func newModelsCurrentCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the currently active model",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Config == nil {
				return fmt.Errorf("config not loaded")
			}
			fmt.Fprintln(cmd.OutOrStdout(), app.Config.Model)
			return nil
		},
	}
}
