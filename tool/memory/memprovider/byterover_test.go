package memprovider

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

// TestByteroverQueryUsesInjectedCommand substitutes execCommand with a
// shim that runs `/bin/echo` (or equivalent) so the test doesn't need
// the real brv CLI.
func TestByteroverQueryUsesInjectedCommand(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Rebuild args as a single string that echo prints.
		rebuilt := append([]string{"BRV:"}, args...)
		return exec.CommandContext(ctx, "echo", rebuilt...)
	}

	p := NewByterover(config.ByteroverConfig{BrvPath: "/usr/bin/fake-brv"})
	_ = p.Initialize(context.Background(), "sess-1")
	// Initialize sets brvPath to the configured BrvPath (no LookPath).
	if p.brvPath == "" {
		t.Fatalf("expected brvPath to be set")
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)

	args, _ := json.Marshal(map[string]string{"query": "hermes"})
	res, err := reg.Dispatch(context.Background(), "brv_query", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(res, "BRV:") || !strings.Contains(res, "hermes") {
		t.Errorf("unexpected output: %s", res)
	}
}
