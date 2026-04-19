package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveToPath_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Skills = SkillsConfig{
		Disabled: []string{"alpha"},
		PlatformDisabled: map[string][]string{
			"cli": {"beta"},
		},
	}
	if err := SaveToPath(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	back, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(back.Skills.Disabled) != 1 || back.Skills.Disabled[0] != "alpha" {
		t.Errorf("disabled = %v", back.Skills.Disabled)
	}
	if got := back.Skills.PlatformDisabled["cli"]; len(got) != 1 || got[0] != "beta" {
		t.Errorf("platform_disabled = %v", back.Skills.PlatformDisabled)
	}
}

func TestSaveToPath_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "sub", "config.yaml")
	if err := SaveToPath(path, Default()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
}
