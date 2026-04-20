package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestBatchCmd_RequiresConfigArg(t *testing.T) {
	cmd := newBatchCmd(&App{})
	cmd.SetArgs([]string{"run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Error("expected error when config missing")
	}
}

func TestBatchCmd_CheckValidatesConfig(t *testing.T) {
	dir := t.TempDir()
	dataset := filepath.Join(dir, "d.jsonl")
	if err := os.WriteFile(dataset, []byte(`{"id":"x","prompt":"hi"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "cfg.yaml")
	yaml := []byte(`model: fake/model
dataset_file: ` + dataset + `
output_dir: ` + filepath.Join(dir, "out") + `
`)
	if err := os.WriteFile(cfgPath, yaml, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newBatchCmd(&App{})
	cmd.SetArgs([]string{"run", cfgPath, "--check"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v\nout: %s", err, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("batch: config OK")) {
		t.Errorf("expected OK line, got %q", out.String())
	}
}

func TestBatchCmd_ResolveProviderForModel(t *testing.T) {
	app := &App{
		Config: &config.Config{
			Providers: map[string]config.ProviderConfig{
				"openai": {Provider: "openai", APIKey: "sk-test", Model: "gpt-4o"},
			},
		},
		ConfigPath: "/tmp/fake.yaml",
	}
	p, err := resolveProviderForModel(app, "openai/gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if p.Provider != "openai" || p.Model != "gpt-4o-mini" || p.APIKey != "sk-test" {
		t.Errorf("resolved = %+v", p)
	}
}

func TestBatchCmd_ResolveProviderForModel_UnknownProvider(t *testing.T) {
	app := &App{
		Config:     &config.Config{Providers: map[string]config.ProviderConfig{}},
		ConfigPath: "/tmp/fake.yaml",
	}
	if _, err := resolveProviderForModel(app, "nosuch/foo"); err == nil {
		t.Error("expected error for unknown provider")
	}
}

