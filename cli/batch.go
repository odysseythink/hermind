// cli/batch.go
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/odysseythink/hermind/agent/batch"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/pantheonadapter"
	"github.com/odysseythink/hermind/rl/collector"
	"github.com/odysseythink/hermind/rl/trajectory"
	"github.com/spf13/cobra"
)

// newBatchCmd creates the "hermind batch" subcommand tree.
func newBatchCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run the agent across a dataset in parallel",
	}
	cmd.AddCommand(newBatchRunCmd(app))
	return cmd
}

func newBatchRunCmd(app *App) *cobra.Command {
	var (
		resume        bool
		check         bool
		trajectoryOut string
	)
	c := &cobra.Command{
		Use:   "run <config.yaml>",
		Short: "Run a batch described by the given YAML config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := batch.LoadConfig(args[0])
			if err != nil {
				return err
			}
			cfg.Resume = resume

			// Attach the RL trajectory collector if --trajectory-out is set.
			// Accepts either a bare path or a "file:/path" spec so the flag
			// can be extended to other sinks (gRPC, S3) without a breaking
			// change.
			var coll *collector.Collector
			if trajectoryOut != "" {
				sinkPath := parseTrajectorySpec(trajectoryOut)
				fsink, err := trajectory.NewFileSink(sinkPath)
				if err != nil {
					return fmt.Errorf("batch: open trajectory sink: %w", err)
				}
				coll = collector.New(fsink, trajectory.Meta{
					Environment: cfg.Environment,
					Model:       cfg.Model,
				})
				defer coll.Close()
				fmt.Fprintf(cmd.OutOrStdout(), "batch: trajectory sink = %s\n", sinkPath)
			}

			if check {
				items, err := batch.ReadDataset(cfg.DatasetFile, cfg.MaxItems)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"batch: config OK (model=%s, items=%d, workers=%d, out=%s)\n",
					cfg.Model, len(items), cfg.NumWorkers, cfg.OutputDir)
				return nil
			}

			provCfg, err := resolveProviderForModel(app, cfg.Model)
			if err != nil {
				return err
			}
			m, err := pantheonadapter.BuildModel(cmd.Context(), provCfg)
			if err != nil {
				return err
			}

			runner := batch.NewRunner(cfg, m)
			if coll != nil {
				runner.WithSink(coll)
			}
			return runner.Run(cmd.Context())
		},
	}
	c.Flags().BoolVar(&resume, "resume", false, "skip items already present in the checkpoint file")
	c.Flags().BoolVar(&check, "check", false, "validate the config + dataset and exit")
	c.Flags().StringVar(&trajectoryOut, "trajectory-out", "",
		`write Tinker-compatible RL episodes as JSONL (e.g. "/path/episodes.jsonl" or "file:/path/episodes.jsonl")`)
	return c
}

// parseTrajectorySpec accepts either a bare path or a "kind:value"
// spec. Today only "file:" is recognized; unknown prefixes are treated
// as paths so users can drop in gs:// or s3:// URLs later without the
// flag fighting them.
func parseTrajectorySpec(spec string) string {
	if strings.HasPrefix(spec, "file:") {
		return strings.TrimPrefix(spec, "file:")
	}
	return spec
}

// splitModelRef splits "anthropic/claude-opus-4-6" into ("anthropic",
// "claude-opus-4-6"). Refs without a slash are treated as the provider
// name with an empty model.
func splitModelRef(ref string) (string, string) {
	if i := strings.Index(ref, "/"); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}

// resolveProviderForModel maps a "<name>/<model>" string (e.g.
// "bedrock/anthropic.claude-opus-4-v1:0") to a config.ProviderConfig
// drawn from the loaded hermind config. The model portion after the
// first "/" overrides the config's default Model. For anthropic, falls
// back to the ANTHROPIC_API_KEY env var if api_key is not set — matches
// buildPrimaryProvider's behaviour.
func resolveProviderForModel(app *App, modelRef string) (config.ProviderConfig, error) {
	name, model := splitModelRef(modelRef)
	if app == nil || app.Config == nil {
		return config.ProviderConfig{}, fmt.Errorf("batch: no hermind config loaded (provider %q)", name)
	}
	p, ok := app.Config.Providers[name]
	if !ok {
		return config.ProviderConfig{}, fmt.Errorf("batch: provider %q not configured in %s", name, app.ConfigPath)
	}
	if p.Provider == "" {
		p.Provider = name
	}
	if model != "" {
		p.Model = model
	}
	if name == "anthropic" && p.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			p.APIKey = envKey
		}
	}
	return p, nil
}
