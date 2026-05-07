# Persistent Web Server Port

**Date:** 2026-05-07

## Problem

Currently, `hermind` and `hermind web` pick a random port in `[30000, 40000)` on every startup. The chosen port is never persisted, so:

- Browser bookmarks to the web UI break after every restart.
- External integrations (Obsidian plugin, browser extensions) must re-discover the port each time.
- `hermind run` hardcodes `127.0.0.1:9119`, which is inconsistent with the other commands.

## Goal

1. On the **first** startup (when no port is configured), randomly select an available port, bind to it, and persist the address to the config file.
2. On **subsequent** startups, read the persisted address from the config file and reuse it.
3. The `--addr` CLI flag always takes highest priority and is **not** written to config.
4. Apply this behavior uniformly to `hermind`, `hermind web`, and `hermind run`.

## Design

### Configuration Change

Add an `Addr` field to `WebConfig` in `config/config.go`:

```go
type WebConfig struct {
    // ... existing fields ...
    DisableWebFetch bool         `yaml:"disable_web_fetch,omitempty"`
    Search          SearchConfig `yaml:"search,omitempty"`
    Addr            string       `yaml:"addr,omitempty"` // NEW
}
```

An empty string means "no port assigned yet"; trigger random selection.

### Core Startup Flow

All three commands delegate to `runWeb()` in `cli/web.go`. The binding logic becomes:

```
if opts.Addr != "" {
    // CLI override — highest priority, do NOT persist
    ln = net.Listen("tcp", opts.Addr)
} else if cfg.Web.Addr != "" {
    // Previously persisted address
    ln = net.Listen("tcp", cfg.Web.Addr)
    if err == EADDRINUSE {
        // Port taken (e.g. by another process or another hermind instance).
        // Clear the stale address and fall back to random selection.
        cfg.Web.Addr = ""
        ln = listenRandomLocalhost()
        cfg.Web.Addr = ln.Addr().String()
        saveConfig(cfg)
    }
} else {
    // First start — random port
    ln = listenRandomLocalhost()
    cfg.Web.Addr = ln.Addr().String()
    saveConfig(cfg)
}
```

`saveConfig` is best-effort: if it fails (disk full, permission denied), log a warning and continue. The server starts successfully; the next startup will simply randomize again.

### Command Adaptations

| Command | Current default `--addr` | New default `--addr` | Notes |
|---|---|---|---|
| `hermind` (bare) | `""` | `""` | Unchanged; now persists on first start. |
| `hermind web` | `""` | `""` | Same as above. |
| `hermind run` | `"127.0.0.1:9119"` | `""` | No longer hard-coded. |

In `cli/run.go`, remove the line `opts.Addr = "127.0.0.1:9119"` so that `runWeb()` handles the address uniformly.

### Edge Cases

| Scenario | Behavior |
|---|---|
| Config `addr` already in use | Clear config field, randomize again, save new value. |
| 50 random port attempts all fail | Return error, abort startup (unchanged from today). |
| Config save fails | Warn log, continue startup. |
| User manually edits `web.addr` to bad value | `net.Listen` errors → treat as "port in use" → clear and randomize. |
| Multiple instances share one config | Second instance sees `EADDRINUSE` → falls back to its own random port. |

### Backward Compatibility

- Existing `config.yaml` files lack `web.addr` → treated as empty → first start randomizes and writes it. **No breaking change.**
- `hermind run` no longer defaults to `:9119`. This is an intentional behavioral change per the "all commands unified" requirement.

## Files to Modify

1. `config/config.go` — add `Addr` to `WebConfig`
2. `cli/web.go` — update `runWeb()` binding logic, add config persistence
3. `cli/run.go` — remove hard-coded `:9119`
4. `config/loader_test.go` / `config/config_test.go` — add/assert the new field if needed
