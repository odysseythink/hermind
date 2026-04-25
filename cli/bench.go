package cli

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/benchmark"
)

// newBenchCmd returns the `hermind bench` cobra subcommand. The three
// sub-subcommands are scaffolds that wire config flags and call the
// benchmark package; live provider wiring for presets is an explicit
// follow-up (see benchmark package tests for programmatic usage).
func newBenchCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Local A/B evaluation of hermind config presets",
	}
	cmd.AddCommand(newBenchGenerateCmd(app))
	cmd.AddCommand(newBenchRunCmd(app))
	cmd.AddCommand(newBenchJudgeCmd(app))
	cmd.AddCommand(newBenchReplayCmd(app))
	return cmd
}

func newBenchGenerateCmd(app *App) *cobra.Command {
	var (
		count int
		seed  int64
		out   string
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a synthetic dataset via aux LLM",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = context.Background
			return fmt.Errorf("hermind bench generate: not yet wired to live providers — write the dataset file manually (one JSON meta line + one item per line) or call benchmark.Generate programmatically")
		},
	}
	cmd.Flags().IntVar(&count, "count", 50, "number of items")
	cmd.Flags().Int64Var(&seed, "seed", 42, "generation seed")
	cmd.Flags().StringVar(&out, "out", ".hermind/benchmark/dataset.jsonl", "output path")
	return cmd
}

func newBenchRunCmd(app *App) *cobra.Command {
	var (
		datasetPath string
		outDir      string
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run presets against the dataset (stub — wire PresetRunners in code)",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("hermind bench run: stub — preset runners require app-level wiring. See benchmark.Run usage in tests.")
			return benchmark.Run(context.Background(), benchmark.RunConfig{
				DatasetPath: datasetPath,
				OutDir:      outDir,
				Presets: map[string]benchmark.PresetRunner{
					// Placeholder: callers wire actual preset runners here.
					// Each runner: func(ctx context.Context, item benchmark.Item) (*benchmark.RunRecord, error)
				},
			})
		},
	}
	cmd.Flags().StringVar(&datasetPath, "dataset", ".hermind/benchmark/dataset.jsonl", "dataset path")
	cmd.Flags().StringVar(&outDir, "out", ".hermind/benchmark/runs", "run output dir")
	return cmd
}

func newBenchJudgeCmd(app *App) *cobra.Command {
	var (
		runDir string
		out    string
	)
	cmd := &cobra.Command{
		Use:   "judge",
		Short: "Score an existing run dir (stub — providers must be wired)",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("hermind bench judge: stub — providers require app-level wiring.")
			if err := benchmark.JudgeAll(context.Background(), benchmark.JudgeConfig{
				RunDir: runDir,
			}); err != nil {
				return err
			}
			return benchmark.Render(context.Background(), benchmark.RenderConfig{
				RunDir: runDir, OutPath: out,
			})
		},
	}
	cmd.Flags().StringVar(&runDir, "run-dir", ".hermind/benchmark/runs", "run output dir")
	cmd.Flags().StringVar(&out, "out", ".hermind/benchmark/report.md", "markdown report path")
	return cmd
}
