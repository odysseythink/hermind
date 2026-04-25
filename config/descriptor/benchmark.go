package descriptor

// Benchmark mirrors config.BenchmarkConfig including the nested
// ReplayConfig sub-tree. Top-level fields (dataset_size, seed,
// judge_model, out_dir) come first, then replay.* dotted-path fields.
//
// NOTE: as of 2026-04-25 the bench replay CLI commands at
// cli/replay.go do NOT yet consume cfg.Benchmark.Replay.* — they
// hardcode their own defaults. Descriptor `Default` values below
// MATCH the CLI hardcoded defaults so the form reflects today's
// runtime behavior. When the CLI-wiring follow-up lands, both
// continue to agree.
//
// Mode and Judge enum values are the literal runtime strings (verified
// against replay/dataset.go:40 and replay/judge.go:37). In particular,
// "rubric+pairwise" includes the literal '+'.
func init() {
	Register(Section{
		Key:     "benchmark",
		Label:   "Benchmark",
		Summary: "Defaults for `hermind bench` (skills) and `hermind bench replay` (memory replay).",
		GroupID: "advanced",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			// BenchmarkConfig top-level
			{
				Name:    "dataset_size",
				Label:   "Dataset size",
				Help:    "Number of synthetic items per benchmark run. Default 50.",
				Kind:    FieldInt,
				Default: 50,
			},
			{
				Name:    "seed",
				Label:   "Random seed",
				Help:    "Deterministic seed for dataset generation. Default 42.",
				Kind:    FieldInt,
				Default: 42,
			},
			{
				Name:    "judge_model",
				Label:   "Judge model (default)",
				Help:    "Default judge model. Empty falls back to the auxiliary model, then the primary model. Used by every bench subtree unless overridden.",
				Kind:    FieldString,
				Default: "",
			},
			{
				Name:    "out_dir",
				Label:   "Output directory",
				Help:    "Where bench artifacts are written. Default .hermind/benchmark.",
				Kind:    FieldString,
				Default: ".hermind/benchmark",
			},

			// Replay sub-tree (defaults match cli/replay.go hardcoded values)
			{
				Name:    "replay.default_mode",
				Label:   "Replay default mode",
				Help:    "Replay invocation mode. cold = run-without-history; contextual = run-with-history.",
				Kind:    FieldEnum,
				Enum:    []string{"cold", "contextual"},
				Default: "cold",
			},
			{
				Name:    "replay.default_history_cap",
				Label:   "Replay history cap",
				Help:    "Max prior turns injected into a contextual replay. 0 means \"use the built-in default of 20 when mode is contextual\".",
				Kind:    FieldInt,
				Default: 20,
			},
			{
				Name:    "replay.default_judge",
				Label:   "Replay default judge mode",
				Help:    "Judge mode for replay outputs. none = skip judging; pairwise = compare current vs baseline; rubric+pairwise = pairwise plus a rubric score pass.",
				Kind:    FieldEnum,
				Enum:    []string{"none", "pairwise", "rubric+pairwise"},
				Default: "none",
			},
			{
				Name:    "replay.out_dir",
				Label:   "Replay output directory",
				Help:    "Where replay datasets and run records land. Default .hermind/replay.",
				Kind:    FieldString,
				Default: ".hermind/replay",
			},
			{
				Name:    "replay.judge_model",
				Label:   "Replay judge model (override)",
				Help:    "Override judge model for replay only. Empty falls back to the parent benchmark.judge_model.",
				Kind:    FieldString,
				Default: "",
			},
		},
	})
}
