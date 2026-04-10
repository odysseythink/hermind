// cli/app.go
package cli

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/storage"
)

// App bundles the shared resources (config, storage) across cobra commands.
type App struct {
	Config  *config.Config
	Storage storage.Storage
}

// NewApp constructs an App by loading config. Storage is opened lazily
// by the command that needs it.
func NewApp() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &App{Config: cfg}, nil
}

// Close releases all held resources.
func (a *App) Close() error {
	if a.Storage != nil {
		return a.Storage.Close()
	}
	return nil
}
