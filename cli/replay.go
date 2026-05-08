package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/benchmark"
	"github.com/odysseythink/hermind/replay"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func newBenchReplayCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "replay",
		Short: "Re-run real historical user turns from state.db (replay mode)",
	}
	root.AddCommand(newBenchReplayGenerateCmd(app))
	root.AddCommand(newBenchReplayRunCmd(app))
	root.AddCommand(newBenchReplayJudgeCmd(app))
	root.AddCommand(newBenchReplayReportCmd(app))
	return root
}

func newBenchReplayGenerateCmd(app *App) *cobra.Command {
	var (
		mode       string
		historyCap int
		outDir     string
		statePath  string
		limit      int
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Walk state.db into a replay JSONL dataset",
		RunE: func(_ *cobra.Command, _ []string) error {
			if mode == "" {
				mode = "cold"
			}
			if outDir == "" {
				outDir = ".hermind/replay"
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			if statePath == "" {
				statePath = ".hermind/state.db"
			}
			store, err := sqlite.Open(statePath)
			if err != nil {
				return fmt.Errorf("open state.db: %w", err)
			}
			defer store.Close()
			if err := store.Migrate(); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			outPath := filepath.Join(outDir, "dataset.jsonl")
			cfg := replay.GenerateConfig{
				Mode:         mode,
				HistoryCap:   historyCap,
				UserMsgLimit: limit,
				OutPath:      outPath,
			}
			if err := replay.Generate(context.Background(), store, cfg); err != nil {
				return err
			}
			log.Printf("replay: dataset written to %s", outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "", "cold | contextual (default cold)")
	cmd.Flags().IntVar(&historyCap, "history-cap", 0, "max history per item (default 20 in contextual)")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "output directory (default .hermind/replay)")
	cmd.Flags().StringVar(&statePath, "state", "", "path to state.db (default .hermind/state.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap items generated (0 = no cap)")
	return cmd
}

func newBenchReplayRunCmd(app *App) *cobra.Command {
	var (
		dataset string
		outDir  string
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Re-run replay items against the current preset",
		RunE: func(_ *cobra.Command, _ []string) error {
			if dataset == "" {
				dataset = ".hermind/replay/dataset.jsonl"
			}
			if outDir == "" {
				outDir = ".hermind/replay/runs"
			}
			log.Printf("hermind bench replay run: stub — preset runners require app-level wiring. See replay.LoadDataset and benchmark.Run usage in tests.")
			return benchmark.Run(context.Background(), benchmark.RunConfig{
				DatasetPath: dataset,
				OutDir:      outDir,
				Presets:     map[string]benchmark.PresetRunner{},
				LoaderFn:    replay.LoadDataset,
			})
		},
	}
	cmd.Flags().StringVar(&dataset, "dataset", "", "replay dataset path")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "output directory")
	return cmd
}

func newBenchReplayJudgeCmd(app *App) *cobra.Command {
	var (
		runDir string
		mode   string
	)
	cmd := &cobra.Command{
		Use:   "judge",
		Short: "Score replay run records (none | pairwise | rubric+pairwise)",
		RunE: func(_ *cobra.Command, _ []string) error {
			if runDir == "" {
				runDir = ".hermind/replay/runs"
			}
			if mode == "" {
				mode = "none"
			}
			log.Printf("hermind bench replay judge: stub — aux provider requires app-level wiring.")
			return replay.JudgeAll(context.Background(), runDir, replay.Mode(mode), nil)
		},
	}
	cmd.Flags().StringVar(&runDir, "run-dir", "", "directory containing records.jsonl + dataset.jsonl")
	cmd.Flags().StringVar(&mode, "mode", "", "none | pairwise | rubric+pairwise")
	return cmd
}

func newBenchReplayReportCmd(app *App) *cobra.Command {
	var (
		runDir string
		mode   string
		full   bool
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render replay results as side-by-side markdown + JSON",
		RunE: func(_ *cobra.Command, _ []string) error {
			if runDir == "" {
				runDir = ".hermind/replay/runs"
			}
			if mode == "" {
				mode = "none"
			}
			outPath := filepath.Join(runDir, "report.md")
			return replay.Render(context.Background(), runDir, replay.RenderOptions{
				OutPath: outPath,
				Mode:    replay.Mode(mode),
				Full:    full,
			})
		},
	}
	cmd.Flags().StringVar(&runDir, "run-dir", "", "directory containing records + judge artifacts")
	cmd.Flags().StringVar(&mode, "mode", "", "none | pairwise | rubric+pairwise")
	cmd.Flags().BoolVar(&full, "full", false, "include all items (default: regressions only)")
	return cmd
}
