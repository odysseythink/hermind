// Package batch runs hermind agents against a dataset of prompts in
// parallel and saves the resulting trajectories. It mirrors the
// Python batch_runner.py feature set (MVP scope — no eval harness,
// no trajectory compression). The package is the foundation for the
// RL trajectory bridge; see the TrajectorySink hook for the integration
// point that the rl/ package plugs into.
package batch

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config describes a single batch run. YAML keys match the Python
// datagen-config-examples/*.yaml layout so configs are portable.
type Config struct {
	// Required
	Model       string `yaml:"model"`        // e.g. "openrouter/meta-llama/llama-3-70b"
	DatasetFile string `yaml:"dataset_file"` // JSONL path; each line must have at least a "prompt" field
	OutputDir   string `yaml:"output_dir"`   // directory to write trajectories + checkpoint into

	// Optional — workload shape
	Environment string   `yaml:"environment,omitempty"` // cosmetic label stored in trajectory metadata
	Toolsets    []string `yaml:"toolsets,omitempty"`    // names resolved against the tool registry (reserved, unused in MVP)
	NumWorkers  int      `yaml:"num_workers,omitempty"` // default 1
	BatchSize   int      `yaml:"batch_size,omitempty"`  // default 1 — passed through as metadata
	MaxItems    int      `yaml:"max_items,omitempty"`   // 0 means "no cap"
	MaxTurns    int      `yaml:"max_turns,omitempty"`   // per-example cap, reserved for when Engine loop is wired in
	MaxTokens   int      `yaml:"max_tokens,omitempty"`  // MaxTokens passed to the provider; default 4096

	// Optional — per-run system prompt additions
	EphemeralSystemPrompt string `yaml:"ephemeral_system_prompt,omitempty"`

	// Resume tells the runner to skip IDs already present in the
	// checkpoint file. The flag is wired from the CLI layer; it has
	// no YAML representation on purpose (it's a per-invocation knob).
	Resume bool `yaml:"-"`
}

// LoadConfig reads a YAML file, applies defaults, and validates the
// required fields.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("batch: read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("batch: parse %s: %w", path, err)
	}
	ApplyDefaults(&cfg)
	if err := Validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ApplyDefaults fills zero-valued fields with the package defaults.
// Exported so callers constructing a Config programmatically (e.g. in
// tests or the RL bridge) get the same behaviour as LoadConfig.
func ApplyDefaults(cfg *Config) {
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = 1
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
}

// Validate reports the first missing required field, if any.
func Validate(cfg *Config) error {
	if cfg.Model == "" {
		return errors.New("batch: config: model is required")
	}
	if cfg.DatasetFile == "" {
		return errors.New("batch: config: dataset_file is required")
	}
	if cfg.OutputDir == "" {
		return errors.New("batch: config: output_dir is required")
	}
	return nil
}
