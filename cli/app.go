// cli/app.go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

// App bundles the shared resources (config, storage) across cobra commands.
type App struct {
	Config     *config.Config
	ConfigPath string
	Storage    storage.Storage
}

// NewApp constructs an App by loading config. Storage is opened lazily
// by the command that needs it.
//
// When no config file exists, a minimal default is written to the
// default path and the user is pointed at `hermind web` for
// configuration. The legacy bubbletea first-run TUI has been removed.
func NewApp() (*App, error) {
	path, err := defaultConfigPath()
	if err != nil {
		return nil, err
	}

	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		if err := writeDefaultConfig(path); err != nil {
			return nil, fmt.Errorf("write default config: %w", err)
		}
		fmt.Fprintf(os.Stderr,
			"hermind: wrote default config to %s — run `hermind web` to configure providers.\n",
			path,
		)
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

// defaultConfigPath resolves ~/.hermind/config.yaml with the
// HERMIND_HOME override taken into account.
func defaultConfigPath() (string, error) {
	if v := os.Getenv("HERMIND_HOME"); v != "" {
		return filepath.Join(v, "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hermind", "config.yaml"), nil
}

// writeDefaultConfig creates the parent directory (if needed) and
// writes a minimal YAML containing defaults.
func writeDefaultConfig(path string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	cfg := config.Default()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
