package cli

import (
	"fmt"
	"os"
	"path/filepath"

	configui "github.com/odysseythink/hermind/cli/ui/config"
	"github.com/spf13/cobra"
)

func newConfigCmd(app *App) *cobra.Command {
	var web bool
	var port int
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Open the configuration editor (TUI by default, --web for browser)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := defaultConfigPath()
			if err != nil {
				return err
			}
			if web {
				return fmt.Errorf("--web: not yet implemented (wired in Task 13)")
			}
			return configui.Run(path)
		},
	}
	cmd.Flags().BoolVar(&web, "web", false, "open the browser editor instead of the TUI")
	cmd.Flags().IntVar(&port, "port", 7777, "port for the --web editor")
	return cmd
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hermind", "config.yaml"), nil
}
