// cli/run.go
package cli

import (
	"github.com/spf13/cobra"
)

// newRunCmd creates the "hermind run" command. Both "hermind" and
// "hermind run" launch the same REPL.
func newRunCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the interactive REPL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runREPL(cmd.Context(), app)
		},
	}
}
