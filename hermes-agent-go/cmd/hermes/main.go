// cmd/hermes/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nousresearch/hermes-agent/cli"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	cli.Version = Version

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
