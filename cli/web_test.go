package cli

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// safeBuffer wraps bytes.Buffer with a mutex for concurrent read/write
// by the goroutine running the command and the test assertions.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newWebTestApp builds an App with a temp config/storage so tests run
// hermetically.
func newWebTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("model: anthropic/claude-opus-4-6\nstorage:\n  driver: sqlite\n  sqlite_path: "+filepath.Join(dir, "state.db")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func TestWebCmd_BindsAndServesStatus(t *testing.T) {
	app := newWebTestApp(t)
	cmd := newWebCmd(app)
	cmd.SetArgs([]string{"--addr", "127.0.0.1:0", "--no-browser", "--exit-after", "2s"})
	out := &safeBuffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.ExecuteContext(context.Background()) }()

	// Wait up to 2s for the listening line to appear.
	deadline := time.Now().Add(2 * time.Second)
	var addr string
	for time.Now().Before(deadline) {
		for _, l := range strings.Split(out.String(), "\n") {
			if i := strings.Index(l, "http://"); i >= 0 {
				// Take only the first whitespace-delimited token.
				rest := strings.TrimSpace(l[i:])
				if sp := strings.IndexAny(rest, " \t"); sp >= 0 {
					rest = rest[:sp]
				}
				addr = rest
				break
			}
		}
		if addr != "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if addr == "" {
		t.Fatalf("listening URL not found in output: %q", out.String())
	}

	resp, err := http.Get(addr + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// /api/sessions is gone in the single-conversation model — 404 now.
	resp, err = http.Get(addr + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	_ = resp.Body.Close()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("cmd: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("cmd did not exit")
	}
}

// TestHermindRun_DispatchesToWeb verifies `hermind run` now launches
// the same web server as `hermind web` (the TUI entry was removed in
// Phase 3).
func TestHermindRun_DispatchesToWeb(t *testing.T) {
	app := newWebTestApp(t)
	cmd := NewRootCmd(app)
	cmd.SetArgs([]string{"run", "--addr", "127.0.0.1:0", "--no-browser", "--exit-after", "1s"})
	out := &safeBuffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "hermind web listening on") {
		t.Errorf("`run` did not dispatch to web; output=%q", out.String())
	}
}

// TestHermindConfig_Removed confirms `hermind config` is no longer a
// registered subcommand.
func TestHermindConfig_Removed(t *testing.T) {
	app := newWebTestApp(t)
	cmd := NewRootCmd(app)
	cmd.SetArgs([]string{"config"})
	cmd.SetOut(&safeBuffer{})
	cmd.SetErr(&safeBuffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected `hermind config` to be unknown; got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' error, got %v", err)
	}
}
