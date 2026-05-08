package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool/terminal"
	"github.com/spf13/cobra"
)

// newDoctorCmd creates the "hermind doctor" health-check command.
func newDoctorCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks against the current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), app)
		},
	}
}

type doctorCheck struct {
	name string
	fn   func(ctx context.Context, app *App) error
}

func runDoctor(ctx context.Context, app *App) error {
	checks := []doctorCheck{
		{"config file readable", checkConfig},
		{"primary provider builds", checkPrimaryProvider},
		{"storage opens and migrates", checkStorage},
		{"terminal backend executes `true`", checkTerminalBackend},
	}
	failed := 0
	for _, c := range checks {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := c.fn(cctx, app)
		cancel()
		if err != nil {
			fmt.Printf("✗ %s: %v\n", c.name, err)
			failed++
		} else {
			fmt.Printf("✓ %s\n", c.name)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	fmt.Println("\nall checks passed.")
	return nil
}

func checkConfig(ctx context.Context, app *App) error {
	if app.Config == nil {
		return fmt.Errorf("config not loaded")
	}
	if app.Config.Model == "" {
		return fmt.Errorf("model not set")
	}
	return nil
}

func checkPrimaryProvider(ctx context.Context, app *App) error {
	_, _, err := buildPrimaryProvider(app.Config)
	return err
}

func checkStorage(ctx context.Context, app *App) error {
	path := app.Config.Storage.SQLitePath
	if path == "" {
		p, err := config.InstancePath("state.db")
		if err != nil {
			return fmt.Errorf("resolve instance root: %w", err)
		}
		path = p
	}
	// Don't mutate the app's storage — open a throwaway handle.
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	store, err := sqlite.Open(path)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.Migrate()
}

func checkTerminalBackend(ctx context.Context, app *App) error {
	backend := app.Config.Terminal.Backend
	if backend == "" {
		backend = "local"
	}
	t, err := terminal.New(backend, terminal.Config{})
	if err != nil {
		return err
	}
	defer t.Close()
	// Execute an innocuous command to verify the backend is live.
	res, err := t.Execute(ctx, "true", nil)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("backend returned exit %d", res.ExitCode)
	}
	_ = config.Default // keep import stable if fields change
	return nil
}
