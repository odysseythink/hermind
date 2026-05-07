// cli/run.go
package cli

import (
	"github.com/spf13/cobra"
)

// newRunCmd creates `hermind run` — an alias for `hermind web`.
// Exists for backwards compatibility with scripts that invoke the
// historical REPL entry point.
func newRunCmd(app *App) *cobra.Command {
	var opts webRunOptions
	c := &cobra.Command{
		Use:   "run",
		Short: "Start hermind (alias for `web`)",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Out = cmd.OutOrStdout()
			return runWeb(cmd.Context(), app, opts)
		},
	}
	c.Flags().StringVar(&opts.Addr, "addr", "", "bind address")
	c.Flags().BoolVar(&opts.NoBrowser, "no-browser", false, "do not open the browser automatically")
	c.Flags().DurationVar(&opts.ExitAfter, "exit-after", 0, "exit after the given duration")
	return c
}
