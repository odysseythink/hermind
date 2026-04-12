package rl

import (
	"path/filepath"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Tokenizer != "Qwen/Qwen3-8B" {
		t.Errorf("tokenizer = %q", cfg.Tokenizer)
	}
	if cfg.MaxWorkers != 2048 {
		t.Errorf("max_workers = %d", cfg.MaxWorkers)
	}
}

func TestConfigIsLocked(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.IsLocked("tokenizer") {
		t.Error("tokenizer should be locked")
	}
	if !cfg.IsLocked("max_workers") {
		t.Error("max_workers should be locked")
	}
	if cfg.IsLocked("lora_rank") {
		t.Error("lora_rank should not be locked")
	}
	if cfg.IsLocked("learning_rate") {
		t.Error("learning_rate should not be locked")
	}
}

func TestConfigSetEditable(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Set("lora_rank", 64); err != nil {
		t.Fatalf("Set lora_rank: %v", err)
	}
	if cfg.LoraRank != 64 {
		t.Errorf("lora_rank = %d", cfg.LoraRank)
	}
}

func TestConfigSetLockedFails(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Set("tokenizer", "other"); err == nil {
		t.Error("expected error")
	}
}

func TestConfigSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := DefaultConfig()
	cfg.LoraRank = 64
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.LoraRank != 64 {
		t.Errorf("lora_rank = %d", loaded.LoraRank)
	}
	if loaded.Tokenizer != "Qwen/Qwen3-8B" {
		t.Errorf("tokenizer = %q", loaded.Tokenizer)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LoraRank = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error")
	}
}
