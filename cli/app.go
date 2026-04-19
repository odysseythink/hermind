// cli/app.go
package cli

import (
	"errors"
	"fmt"
	"os"

	configui "github.com/odysseythink/hermind/cli/ui/config"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

// App bundles the shared resources (config, storage) across cobra commands.
type App struct {
	Config     *config.Config
	ConfigPath string // absolute path the config was loaded from
	Storage    storage.Storage
}

// NewApp constructs an App by loading config. Storage is opened lazily
// by the command that needs it.
func NewApp() (*App, error) {
	path, err := defaultConfigPath()
	if err != nil {
		return nil, err
	}

	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "no config found — launching first-run setup...")
		if err := configui.RunFirstRun(path); err != nil {
			return nil, fmt.Errorf("first-run setup: %w", err)
		}
	}

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		return nil, err
	}
	return &App{Config: cfg, ConfigPath: path}, nil
}

// Close releases all held resources.
func (a *App) Close() error {
	if a.Storage != nil {
		return a.Storage.Close()
	}
	return nil
}
