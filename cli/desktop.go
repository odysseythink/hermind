package cli

import (
	"github.com/spf13/cobra"
)

// newDesktopCmd builds the `hermind desktop` subcommand. It reuses runWeb
// with the browser disabled so the desktop client can connect to the backend.
func newDesktopCmd(app *App) *cobra.Command {
	var opts webRunOptions
	c := &cobra.Command{
		Use:   "desktop",
		Short: "Start backend server for desktop client",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.NoBrowser = true
			opts.Out = cmd.OutOrStdout()
			return runWeb(cmd.Context(), app, opts)
		},
	}
	c.Flags().StringVar(&opts.Addr, "addr", "",
		"bind address; empty = random port in [30000,40000) on 127.0.0.1")
	c.Flags().DurationVar(&opts.ExitAfter, "exit-after", 0,
		"exit after the given duration (0 = run until Ctrl-C)")
	return c
}
