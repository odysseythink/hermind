// cli/acp.go
//
// `hermind acp` — run hermind as an ACP stdio server. The subcommand
// reads newline-delimited JSON-RPC 2.0 frames from stdin, dispatches
// them, and writes responses to stdout. Logs are routed elsewhere (the
// logging package targets stderr) so they never corrupt the wire.
package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway/acp/stdio"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// newACPCmd returns the "hermind acp" subcommand.
func newACPCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "acp",
		Short: "Run hermind as an ACP stdio server (editor integration)",
		Long: `acp runs hermind as a newline-delimited JSON-RPC 2.0 server on
stdin/stdout. Editors that speak the Agent Client Protocol (Zed,
Cursor, VS Code plugins) can spawn this subcommand and drive the
agent through session/new, session/prompt, and session/cancel
requests. Logs go to stderr and never corrupt the protocol frames
on stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureStorage(app); err != nil {
				return err
			}

			// Stamp the build-time version into the initialize response.
			stdio.AgentVersion = Version

			handlers := &stdio.Handlers{
				Sessions: stdio.NewSessionManager(app.Storage),
				Factory: func(model string) (provider.Provider, error) {
					cfg := resolveProviderConfig(app.Config, model)
					return factory.New(cfg)
				},
				AgentCfg: app.Config.Agent,
			}
			srv := stdio.NewServer(handlers)
			return srv.Run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

// resolveProviderConfig maps a "<name>/<model>" ref to the
// ProviderConfig from cfg.Providers, overriding the model name when
// one is supplied. When cfg is nil (test fixtures) or has no matching
// entry, the returned config is the zero value — good enough for
// factory.New to return a clean error the CLI can surface.
func resolveProviderConfig(cfg *config.Config, modelRef string) config.ProviderConfig {
	if cfg == nil {
		return config.ProviderConfig{}
	}
	name, model := splitModelRef(modelRef)
	p := cfg.Providers[name]
	// Default the provider name to the portion before the slash if it
	// wasn't explicitly set (legacy configs).
	if p.Provider == "" {
		p.Provider = name
	}
	if model != "" {
		p.Model = model
	}
	return p
}

// splitModelRef splits "anthropic/claude-opus-4-6" → ("anthropic",
// "claude-opus-4-6"). Refs without a slash are treated as
// provider-only; the caller falls back to the provider's configured
// default model.
func splitModelRef(ref string) (string, string) {
	if i := strings.Index(ref, "/"); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}
