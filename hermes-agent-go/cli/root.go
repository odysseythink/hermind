// cli/root.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is injected at build time via -ldflags "-X main.Version=..."
// It is set from main.go.
var Version = "dev"

// NewRootCmd builds the cobra command tree.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "hermes",
		Short:         "Hermes Agent — Go port of the hermes AI agent framework",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newRunCmd(app),
		newGatewayCmd(app),
		newCronCmd(app),
		newVersionCmd(),
	)

	// Default subcommand: if no args, run the REPL
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return runREPL(cmd.Context(), app)
	}

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "hermes-agent %s\n", Version)
			return nil
		},
	}
}
