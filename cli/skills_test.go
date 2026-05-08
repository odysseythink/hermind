package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestSkillsEnableDisable_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Seed: skills dir with two skills.
	seed := func(name string) {
		p := filepath.Join(dir, "skills", "cat", name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: test\n---\nbody"
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed("alpha")
	seed("beta")

	// Seed config so NewApp does not launch first-run.
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("model: anthropic/test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// Disable alpha via the CLI helper.
	if err := skillsPersistActive(app, "alpha", false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	// Config file now lists alpha as disabled.
	back, err := config.LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(back.Skills.Disabled) != 1 || back.Skills.Disabled[0] != "alpha" {
		t.Errorf("disabled = %v", back.Skills.Disabled)
	}

	// Re-enable alpha — disabled list is now empty.
	if err := skillsPersistActive(app, "alpha", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	back, _ = config.LoadFromPath(cfgPath)
	if len(back.Skills.Disabled) != 0 {
		t.Errorf("expected empty disabled, got %v", back.Skills.Disabled)
	}

	// Unknown skill returns an error and does not mutate config.
	var stderr bytes.Buffer
	if err := skillsPersistActiveWithOut(app, "ghost", true, &stderr); err == nil {
		t.Errorf("expected error for unknown skill")
	}
}
