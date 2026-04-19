// cli/root.go
package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version, Commit, and BuildDate are injected at build time via
// -ldflags "-X main.Version=... -X main.Commit=... -X main.BuildDate=...".
// main.go copies them into these vars before cobra runs.
var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

// NewRootCmd builds the cobra command tree.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "hermind",
		Short:         "Hermind — Go port of the hermes AI agent framework",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}
	root.SetVersionTemplate("hermind {{.Version}}\n")

	root.AddCommand(
		newRunCmd(app),
		newGatewayCmd(app),
		newCronCmd(app),
		newSkillsCmd(app),
		newSetupCmd(app),
		newConfigCmd(app),
		newDoctorCmd(app),
		newAuthCmd(app),
		newModelsCmd(app),
		newProfileCmd(app),
		newPluginsCmd(app),
		newUpgradeCmd(app),
		newRLCmd(app),
		newMCPCmd(app),
		newACPCmd(app),
		newBatchCmd(app),
		newWebCmd(app),
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
			fmt.Fprintf(cmd.OutOrStdout(),
				"hermind %s\n  commit:     %s\n  built:      %s\n  go:         %s\n",
				Version,
				coalesce(Commit, "unknown"),
				coalesce(BuildDate, "unknown"),
				runtime.Version(),
			)
			return nil
		},
	}
}

func coalesce(a, b string) string {
	if a == "" {
		return b
	}
	return a
}
