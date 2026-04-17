package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/storage/sqlite"
)

func newMCPTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &App{Storage: store}
}

func TestMCPCmd_InitializeRoundTrip(t *testing.T) {
	app := newMCPTestApp(t)
	cmd := newMCPCmd(app)
	var serve *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Use == "serve" {
			serve = c
		}
	}
	if serve == nil {
		t.Fatal("serve subcommand missing")
	}
	cmd.SetArgs([]string{"serve"})
	cmd.SetIn(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = serve
	if !strings.Contains(out.String(), `"protocolVersion"`) {
		t.Errorf("got %s", out.String())
	}
}
