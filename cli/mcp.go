package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/mcp/server"
)

func newMCPCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP (Model Context Protocol) server",
	}
	cmd.AddCommand(newMCPServeCmd(app))
	return cmd
}

func newMCPServeCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run hermind as an MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureStorage(app); err != nil {
				return err
			}
			bridge := server.NewEventBridge(app.Storage, 200*time.Millisecond)
			perms := server.NewPermissionQueue()

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			go bridge.Run(ctx)

			srv := server.NewServer(&server.ServerOpts{
				Storage:     app.Storage,
				Events:      bridge,
				Permissions: perms,
			})
			return srv.Run(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}
