package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func newACPTestApp(t *testing.T) *App {
	t.Helper()
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &App{
		Config:  &config.Config{Providers: map[string]config.ProviderConfig{}},
		Storage: store,
	}
}

func TestACPCmd_InitializeRoundTrip(t *testing.T) {
	app := newACPTestApp(t)
	cmd := newACPCmd(app)

	cmd.SetIn(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), `"protocolVersion"`) {
		t.Errorf("missing protocolVersion: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"agentInfo"`) {
		t.Errorf("missing agentInfo: %s", stdout.String())
	}
}

func TestACPCmd_NewSessionPersists(t *testing.T) {
	app := newACPTestApp(t)
	cmd := newACPCmd(app)

	in := `{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"/tmp","model":"stub/model"}}` + "\n"
	cmd.SetIn(strings.NewReader(in))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), `"sessionId"`) {
		t.Fatalf("missing sessionId: %s", stdout.String())
	}
}

func TestSplitModelRef(t *testing.T) {
	cases := []struct {
		in       string
		provider string
		model    string
	}{
		{"anthropic/claude-opus-4-6", "anthropic", "claude-opus-4-6"},
		{"anthropic", "anthropic", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		p, m := splitModelRef(c.in)
		if p != c.provider || m != c.model {
			t.Errorf("splitModelRef(%q) = (%q, %q), want (%q, %q)", c.in, p, m, c.provider, c.model)
		}
	}
}

func TestResolveProviderConfig_NilConfigSafe(t *testing.T) {
	got := resolveProviderConfig(nil, "anthropic/claude")
	if got != (config.ProviderConfig{}) {
		t.Errorf("expected zero value, got %+v", got)
	}
}

func TestResolveProviderConfig_OverridesModel(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {Provider: "anthropic", APIKey: "k", Model: "default"},
		},
	}
	got := resolveProviderConfig(cfg, "anthropic/claude-opus-4-6")
	if got.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q", got.Model)
	}
	if got.Provider != "anthropic" {
		t.Errorf("Provider = %q", got.Provider)
	}
	if got.APIKey != "k" {
		t.Errorf("APIKey dropped: %q", got.APIKey)
	}
}
