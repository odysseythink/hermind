// cli/reloader_test.go
package cli

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/cli/ui"
	"github.com/odysseythink/hermind/config"
)

// TestRuntimeReloaderSwapsOnMtimeChange writes a config file, starts the
// reloader, rewrites the file, and asserts that the RuntimeSnapshot's
// Model field reflects the new config within a few poll cycles.
func TestRuntimeReloaderSwapsOnMtimeChange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("model: first/m1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var ref atomic.Pointer[ui.RuntimeSnapshot]
	ref.Store(&ui.RuntimeSnapshot{Model: "m1"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startRuntimeReloader(ctx, p, &ref, func(cfg *config.Config) (*ui.RuntimeSnapshot, error) {
		return &ui.RuntimeSnapshot{Model: defaultModelFromString(cfg.Model)}, nil
	})

	// Wait a poll tick, then rewrite the file with a newer mtime.
	time.Sleep(reloadPollInterval + 100*time.Millisecond)
	newTime := time.Now().Add(time.Second)
	if err := os.WriteFile(p, []byte("model: second/m2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(p, newTime, newTime)

	// Poll for up to 3s for the snapshot's Model to update.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ref.Load().Model == "m2" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("snapshot Model did not update to m2 within 3s (got %q)", ref.Load().Model)
}

// TestRuntimeReloaderKeepsOldSnapshotOnRebuildError confirms that a
// rebuild failure leaves the previous snapshot in place.
func TestRuntimeReloaderKeepsOldSnapshotOnRebuildError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("model: first/m1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var ref atomic.Pointer[ui.RuntimeSnapshot]
	ref.Store(&ui.RuntimeSnapshot{Model: "m1"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rebuildCalls := 0
	startRuntimeReloader(ctx, p, &ref, func(cfg *config.Config) (*ui.RuntimeSnapshot, error) {
		rebuildCalls++
		return nil, errFake{}
	})

	time.Sleep(reloadPollInterval + 100*time.Millisecond)
	newTime := time.Now().Add(time.Second)
	if err := os.WriteFile(p, []byte("model: second/m2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(p, newTime, newTime)

	time.Sleep(reloadPollInterval * 3)
	if ref.Load().Model != "m1" {
		t.Errorf("snapshot changed on failed rebuild: got %q, want m1", ref.Load().Model)
	}
	if rebuildCalls == 0 {
		t.Error("expected rebuild to be called at least once")
	}
}

type errFake struct{}

func (errFake) Error() string { return "rebuild failed" }
