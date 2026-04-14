// cmd/hermind/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/odysseythink/hermind/cli"
)

// Injected at build time via ldflags.
var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

func main() {
	cli.Version = Version
	cli.Commit = Commit
	cli.BuildDate = BuildDate

	app, err := cli.NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hermes: init: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = app.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := cli.NewRootCmd(app)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "hermes: %v\n", err)
		os.Exit(1)
	}
}
