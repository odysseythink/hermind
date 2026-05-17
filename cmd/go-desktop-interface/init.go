package main

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/cli"
)

var globalApp *cli.App
var globalServer *api.Server
var globalCleanup func()

func initHermind(configPath string) (map[string]string, error) {
	app, err := cli.NewApp()
	if err != nil {
		return nil, fmt.Errorf("new app: %w", err)
	}
	globalApp = app

	if err := cli.EnsureStorage(app); err != nil {
		return nil, fmt.Errorf("ensure storage: %w", err)
	}

	ctx := context.Background()
	deps, cleanup, err := cli.BuildEngineDeps(ctx, app)
	if cleanup != nil {
		globalCleanup = cleanup
	}
	if err != nil {
		// Degraded mode (missing API key) is acceptable for desktop
		fmt.Printf("warning: build engine deps: %v\n", err)
	}

	streams := api.NewMemoryStreamHub()
	srv, err := api.NewServer(&api.ServerOpts{
		Config:       app.Config,
		ConfigPath:   app.ConfigPath,
		InstanceRoot: app.InstanceRoot,
		Storage:      app.Storage,
		Version:      cli.Version,
		Streams:      streams,
		Deps:         deps,
	})
	if err != nil {
		return nil, fmt.Errorf("new server: %w", err)
	}
	globalServer = srv

	return map[string]string{
		"status":       "ok",
		"version":      cli.Version,
		"instanceRoot": app.InstanceRoot,
	}, nil
}
