// cli/reloader.go
package cli

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/odysseythink/hermind/cli/ui"
	"github.com/odysseythink/hermind/config"
)

// reloadPollInterval is how often the reloader polls the config file for
// mtime changes. Polling is simple, cross-platform, and has negligible
// cost for a developer-only dev tool.
const reloadPollInterval = 500 * time.Millisecond

// startRuntimeReloader polls configPath for mtime changes and swaps the
// RuntimeSnapshot pointer when the file is rewritten. Rebuild is called
// with the freshly-parsed config; if it returns an error, the previous
// snapshot stays in place.
//
// Scope: live-reloadable = model + providers (primary, fallbacks, aux) +
// agent config (max_turns, compression). Out of scope: storage driver,
// terminal backend, MCP servers, memory provider, browser provider, tool
// registry — changes to those require restarting the REPL.
func startRuntimeReloader(
	ctx context.Context,
	configPath string,
	ref *atomic.Pointer[ui.RuntimeSnapshot],
	rebuild func(*config.Config) (*ui.RuntimeSnapshot, error),
) {
	go func() {
		var lastMod time.Time
		if info, err := os.Stat(configPath); err == nil {
			lastMod = info.ModTime()
		}
		ticker := time.NewTicker(reloadPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(configPath)
				if err != nil {
					continue
				}
				if !info.ModTime().After(lastMod) {
					continue
				}
				lastMod = info.ModTime()
				cfg, err := config.LoadFromPath(configPath)
				if err != nil {
					slog.WarnContext(ctx, "config reload: parse failed", "err", err)
					continue
				}
				snap, err := rebuild(cfg)
				if err != nil {
					slog.WarnContext(ctx, "config reload: rebuild failed", "err", err)
					continue
				}
				ref.Store(snap)
				slog.InfoContext(ctx, "config reloaded", "model", snap.Model)
			}
		}
	}()
}
