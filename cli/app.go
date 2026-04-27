// cli/app.go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
)

// App bundles the shared resources (config, storage) across cobra commands.
type App struct {
	Config       *config.Config
	ConfigPath   string
	InstanceRoot string
	Storage      storage.Storage
}

// NewApp loads the instance config. First-run behavior:
//   - creates <instance-root>/ if missing
//   - writes a default config.yaml if missing
//   - if ~/.hermind exists and HERMIND_HOME is unset and the instance
//     has not shown the notice yet, prints a one-time stderr hint and
//     touches a marker file so the hint does not repeat.
func NewApp() (*App, error) {
	root, err := config.InstanceRoot()
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(root, config.DefaultConfigFile)

	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("hermind: create instance root %s: %w", root, err)
	}

	if err := skills.EnsureDefaults(filepath.Join(root, "skills")); err != nil {
		fmt.Fprintf(os.Stderr, "hermind: restore default skills: %v\n", err)
	}

	if _, statErr := os.Stat(cfgPath); errors.Is(statErr, os.ErrNotExist) {
		if err := writeDefaultConfig(cfgPath); err != nil {
			return nil, fmt.Errorf("write default config: %w", err)
		}
		fmt.Fprintf(os.Stderr,
			"hermind: wrote default config to %s — run `hermind web` to configure providers.\n",
			cfgPath,
		)
	}

	maybePrintMigrationNotice(root)

	cfg, err := config.LoadFromPath(cfgPath)
	if err != nil {
		return nil, err
	}
	return &App{
		Config:       cfg,
		ConfigPath:   cfgPath,
		InstanceRoot: root,
	}, nil
}

// Close releases all held resources.
func (a *App) Close() error {
	if a.Storage != nil {
		return a.Storage.Close()
	}
	return nil
}

// maybePrintMigrationNotice emits a one-time stderr hint when the user
// has a legacy ~/.hermind/ directory but is now running under a new
// cwd-rooted instance. Respects $HERMIND_HOME (no notice when caller
// has explicitly chosen a root).
func maybePrintMigrationNotice(root string) {
	if os.Getenv("HERMIND_HOME") != "" {
		return
	}
	marker := filepath.Join(root, ".migration_notice_shown")
	if _, err := os.Stat(marker); err == nil {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	legacy := filepath.Join(home, ".hermind")
	if absRoot, errA := filepath.Abs(root); errA == nil {
		if absLegacy, errL := filepath.Abs(legacy); errL == nil && absRoot == absLegacy {
			_ = os.WriteFile(marker, []byte("same-as-legacy\n"), 0o644)
			return
		}
	}
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	fmt.Fprintln(os.Stderr,
		"hermind: legacy config at ~/.hermind/ is not auto-inherited by this instance.")
	fmt.Fprintln(os.Stderr,
		"  If you want to reuse it, copy manually: cp -r ~/.hermind/. "+root+"/")
	_ = os.WriteFile(marker, []byte("shown\n"), 0o644)
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
