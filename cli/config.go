package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	configui "github.com/odysseythink/hermind/cli/ui/config"
	webconfig "github.com/odysseythink/hermind/cli/ui/webconfig"
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
				return serveWebConfig(cmd.Context(), path, port)
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

func serveWebConfig(ctx context.Context, path string, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := "http://" + addr
	fmt.Fprintf(os.Stderr, "config editor: %s\n", url)
	go func() { _ = webconfig.OpenBrowser(url) }()
	return webconfig.Serve(ctx, path, addr)
}
