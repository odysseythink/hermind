package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FullShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := []byte(`environment: web-research
toolsets: [web, file]
num_workers: 4
batch_size: 20
max_items: 500
max_turns: 10
max_tokens: 8000
model: openrouter/meta-llama/llama-3-70b
dataset_file: data/in.jsonl
output_dir: data/out
ephemeral_system_prompt: |
  You are a helpful agent.
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Environment != "web-research" {
		t.Errorf("env = %q", cfg.Environment)
	}
	if cfg.NumWorkers != 4 || cfg.BatchSize != 20 || cfg.MaxItems != 500 {
		t.Errorf("nums = %d/%d/%d", cfg.NumWorkers, cfg.BatchSize, cfg.MaxItems)
	}
	if cfg.MaxTurns != 10 {
		t.Errorf("max_turns = %d", cfg.MaxTurns)
	}
	if cfg.MaxTokens != 8000 {
		t.Errorf("max_tokens = %d", cfg.MaxTokens)
	}
	if cfg.Model != "openrouter/meta-llama/llama-3-70b" {
		t.Errorf("model = %q", cfg.Model)
	}
	if cfg.DatasetFile != "data/in.jsonl" {
		t.Errorf("dataset = %q", cfg.DatasetFile)
	}
	if cfg.OutputDir != "data/out" {
		t.Errorf("out = %q", cfg.OutputDir)
	}
	if len(cfg.Toolsets) != 2 || cfg.Toolsets[0] != "web" {
		t.Errorf("toolsets = %v", cfg.Toolsets)
	}
	if cfg.EphemeralSystemPrompt == "" {
		t.Errorf("prompt empty")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`model: x
dataset_file: in.jsonl
output_dir: out
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NumWorkers != 1 {
		t.Errorf("expected default num_workers=1, got %d", cfg.NumWorkers)
	}
	if cfg.BatchSize != 1 {
		t.Errorf("expected default batch_size=1, got %d", cfg.BatchSize)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected default max_tokens=4096, got %d", cfg.MaxTokens)
	}
}

func TestLoadConfig_ValidatesRequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`output_dir: out`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("expected error when required fields missing")
	}
}

func TestLoadConfig_FileMissing(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Error("expected error for missing file")
	}
}
