package cli

import (
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/rl"
	"github.com/spf13/cobra"
)

func newRLCmd(app *App) *cobra.Command {
	manager := rl.NewManager()
	cmd := &cobra.Command{
		Use:   "rl",
		Short: "Manage RL training runs",
	}
	cmd.AddCommand(
		newRLConfigCmd(app),
		newRLStartCmd(app, manager),
		newRLStatusCmd(manager),
		newRLStopCmd(manager),
		newRLListCmd(manager),
	)
	return cmd
}

func newRLConfigCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show current RL training configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := rl.DefaultConfig()
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newRLStartCmd(app *App, manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "start [python-entrypoint] [args...]",
		Short: "Start a training run",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := manager.Start(cmd.Context(), args[0], args[1:])
			if err != nil {
				return err
			}
			fmt.Printf("Training run started: %s\n", id)
			return nil
		},
	}
}

func newRLStatusCmd(manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "status [run-id]",
		Short: "Check training run status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status := manager.Status(args[0])
			data, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newRLStopCmd(manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "stop [run-id]",
		Short: "Stop a training run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := manager.Stop(args[0]); err != nil {
				return err
			}
			fmt.Printf("Run %s stopped\n", args[0])
			return nil
		},
	}
}

func newRLListCmd(manager *rl.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all training runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			runs := manager.List()
			if len(runs) == 0 {
				fmt.Println("No active runs")
				return nil
			}
			data, _ := json.MarshalIndent(runs, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}
