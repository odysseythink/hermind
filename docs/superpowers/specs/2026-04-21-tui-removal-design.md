# TUI Removal (Phase 3 of 3) — Design Spec

**Date:** 2026-04-21
**Status:** Draft — awaiting user review
**Scope:** Delete the bubbletea-based chat TUI (`cli/ui/`), the TUI config editor (`cli/ui/config/`), and the standalone config web editor (`cli/ui/webconfig/`). Rewire `hermind` and `hermind run` as aliases for `hermind web`. Remove `hermind config`. Drop charmbracelet dependencies.

---

## 1. Why a phase 3

Final step of the three-phase retirement. Depends on both Phase 1 (backend dispatch/cancel endpoints) and Phase 2 (React chat workspace) being merged.

| Phase | State after merge |
|---|---|
| 1 | TUI unchanged. Web has chat over `curl` / API clients. |
| 2 | TUI unchanged. Browser users get chat UI. |
| **3 (this spec)** | TUI gone. `hermind` opens the browser. |

**Do not merge this spec's implementation before Phase 2 ships.** Between Phase 2 landing and Phase 3 landing, the TUI still exists as a working fallback for anyone whose browser refuses to cooperate. After Phase 3 ships, the only chat interface is the web UI.

## 2. Goals

- The bubbletea chat TUI no longer exists.
- The bubbletea config editor no longer exists.
- `cli/ui/webconfig` (the `hermind config --web` mini server) no longer exists; the web UI at `hermind web` covers configuration.
- `hermind` (no args) and `hermind run` both launch the web UI and open the browser — the same behaviour users had when bare `hermind` launched the TUI chat.
- `hermind config` subcommand is gone.
- charmbracelet and bubbletea are no longer dependencies.

## 3. Non-goals

- Changes to `hermind web` itself (Phase 2 owns its UX).
- Changes to the REST API (Phase 1 owns that surface).
- Reworking any other subcommand (`gateway`, `cron`, `skills`, `setup`, `doctor`, `auth`, `models`, `profile`, `plugins`, `upgrade`, `rl`, `mcp`, `version` all unchanged).
- Config file format changes.
- Adding a new headless chat CLI. Users on headless hosts run `hermind web --no-browser` and tunnel the port.
- Deprecation period — `hermind run` becomes an alias immediately, no warning release.
- Removing the `cli/engine_deps.go` built in Phase 1 — it's kept as the shared provider/tool/skills builder.

## 4. Deletion inventory

### 4.1 Whole directories

| Path | Approx lines (incl. tests) | Purpose |
|---|---|---|
| `cli/ui/` | ~1500 | bubbletea chat TUI (model, update, view, renderer, slash, skin, status bar, banner, messages) |
| `cli/ui/config/` | ~525 | bubbletea config editor (model, update, view, editors) |
| `cli/ui/webconfig/` | — | `hermind config --web` mini web server on port 7777. Superseded by `hermind web` at 9119. |

### 4.2 Single files

| File | Lines | Disposition |
|---|---|---|
| `cli/repl.go` | 336 | Delete. Provider/aux/tool/skills construction already extracted into `cli/engine_deps.go` during Phase 1. Phase 3 deletes the TUI-specific shell (program setup, dispatcher, banner prints). |
| `cli/run.go` | 18 | Rewrite — see §5. The filename survives; its body becomes a thin alias for `hermind web`. |
| `cli/stub_provider.go` | 56 | Delete. TUI's degraded-mode fallback. Phase 1 §3 committed web to returning 503 instead of silently booting with a stub. No remaining callers once repl.go is gone. |
| `cli/stub_provider_test.go` | 73 | Delete (tests the file above). |
| `cli/repl_test.go` | 140 | Delete. Provider-construction coverage is in Phase 1's `cli/engine_deps_test.go`. |
| `cli/repl_tool_test.go` | 118 | **Rename** to `cli/engine_e2e_test.go`. Rewrite body to drive the httptest Anthropic server through `sessionrun.Run` instead of through the TUI model. End-to-end tool-use round trip coverage is valuable — keep, don't delete. |
| `cli/config.go` | 52 | Delete. The `hermind config` subcommand is gone. |

### 4.3 Touch-ups in surviving files

| File | Change |
|---|---|
| `cli/app.go` | Remove `configui "github.com/odysseythink/hermind/cli/ui/config"` import and any App fields that only existed for TUI. |
| `cli/root.go` | Remove `newConfigCmd(app)` registration. Keep `newRunCmd(app)` (body rewired in §5). `root.RunE` changes from `runREPL(...)` to `runWebDefault(...)`. |
| `CHANGELOG.md` | BREAKING entry. See §7. |
| `README.md` if present | Rewrite any TUI references to the web UI. |

### 4.4 `go.mod` / `go.sum`

Direct require removals:
```
github.com/charmbracelet/bubbles
github.com/charmbracelet/bubbletea
github.com/charmbracelet/glamour
github.com/charmbracelet/lipgloss
```

Indirect require removals (via `go mod tidy`):
```
github.com/charmbracelet/colorprofile
github.com/charmbracelet/x/ansi
github.com/charmbracelet/x/cellbuf
github.com/charmbracelet/x/exp/slice
github.com/charmbracelet/x/term
```

Possibly more transitives beyond charmbracelet (muesli, sahilm/fuzzy, …) — `go mod tidy` surfaces the full list. `go.sum` shrinks by ~100 lines.

### 4.5 Pre-flight invariant

Before any deletion, run:

```bash
grep -rn 'bubbletea\|lipgloss\|glamour\|bubbles' --include='*.go' .
```

The output must be limited to `cli/ui/*` and `cli/ui/config/*`. Any hit outside those directories means an external consumer exists and the deletion plan needs to shift. If the grep surfaces a hit in `gateway/`, `api/`, `agent/`, etc., **stop and escalate**.

## 5. Entry rewire

Today three command paths lead into runREPL:
- `hermind` (bare) — via `root.RunE` fallback in `cli/root.go:52`
- `hermind run` — via `cli/run.go`
- Nothing else

After Phase 3 all three paths lead into the web server body.

### 5.1 Shared body

Extract `newWebCmd`'s current RunE closure into an exported-in-package function:

```go
// cli/web.go
type webRunOptions struct {
    Addr      string
    NoBrowser bool
    ExitAfter time.Duration
}

func runWeb(ctx context.Context, app *App, opts webRunOptions) error {
    // current body of newWebCmd's RunE — provider build, gatewayctl,
    // api.NewServer, listen, serve.
}

func newWebCmd(app *App) *cobra.Command {
    var opts webRunOptions
    c := &cobra.Command{
        Use:   "web",
        Short: "Start the hermind web UI and REST API",
        Long:  "...",
        RunE:  func(cmd *cobra.Command, args []string) error {
            return runWeb(cmd.Context(), app, opts)
        },
    }
    c.Flags().StringVar(&opts.Addr, "addr", "127.0.0.1:9119", "...")
    c.Flags().BoolVar(&opts.NoBrowser, "no-browser", false, "...")
    c.Flags().DurationVar(&opts.ExitAfter, "exit-after", 0, "...")
    return c
}
```

### 5.2 `hermind run`

```go
// cli/run.go — rewritten
func newRunCmd(app *App) *cobra.Command {
    var opts webRunOptions
    c := &cobra.Command{
        Use:   "run",
        Short: "Start hermind (alias for `web`)",
        RunE:  func(cmd *cobra.Command, args []string) error {
            return runWeb(cmd.Context(), app, opts)
        },
    }
    c.Flags().StringVar(&opts.Addr, "addr", "127.0.0.1:9119", "...")
    c.Flags().BoolVar(&opts.NoBrowser, "no-browser", false, "...")
    c.Flags().DurationVar(&opts.ExitAfter, "exit-after", 0, "...")
    return c
}
```

The flag set duplicates `newWebCmd`. Acceptable — cobra doesn't make flag inheritance trivial for alias-style commands, and the duplication is 3 lines.

### 5.3 `hermind` (bare)

```go
// cli/root.go
root.RunE = func(cmd *cobra.Command, args []string) error {
    return runWeb(cmd.Context(), app, webRunOptions{
        Addr: "127.0.0.1:9119",
    })
}
```

Bare `hermind` doesn't take flags; it always uses the default addr, opens the browser, runs until Ctrl-C. Users who want flag control type `hermind web`.

### 5.4 Final command table

```
hermind                      -> web server + opens browser
hermind run                  -> alias for web (flags supported)
hermind web                  -> primary name (flags: --addr, --no-browser, --exit-after)
hermind gateway ...          -> unchanged
hermind cron ...             -> unchanged
hermind skills ...           -> unchanged
hermind setup                -> unchanged
hermind doctor               -> unchanged
hermind auth ...             -> unchanged
hermind models ...           -> unchanged
hermind profile ...          -> unchanged
hermind plugins ...          -> unchanged
hermind upgrade              -> unchanged
hermind rl ...               -> unchanged
hermind mcp ...              -> unchanged
hermind version              -> unchanged
hermind config               -> REMOVED (use hermind web → Settings)
```

## 6. Tests

### 6.1 Lost coverage

- `cli/repl_test.go` — provider construction / fallback chain / tools registration. **Covered by Phase 1's `cli/engine_deps_test.go`.**
- `cli/stub_provider_test.go` — tests a codepath that no longer exists (degraded stub provider). No replacement needed.

### 6.2 Retained coverage (rewritten)

- `cli/repl_tool_test.go` → `cli/engine_e2e_test.go`. Same httptest Anthropic mock, same tool-use round-trip assertions, but driven through `sessionrun.Run` not through the TUI model.

### 6.3 New coverage

- `cli/web_test.go` additions:
  - `TestBareHermind_RunsWeb` — `cmd.Execute()` with no args → exits without panic, `runWeb` was invoked with default `webRunOptions`. Uses `exit-after: 10ms` to avoid hanging.
  - `TestHermindRun_RunsWeb` — same but `args = []string{"run"}`.
  - `TestHermindWeb_HonoursFlags` — `args = []string{"web", "--addr", "127.0.0.1:0", "--no-browser", "--exit-after", "10ms"}` — bind address honored, no browser open attempted, returns cleanly.
- Optional: `TestHermindConfig_Removed` — `cmd.Execute()` with `args = []string{"config"}` returns a "unknown command" error. Guards against accidentally re-adding.

### 6.4 Regression check after deletion

```bash
go build ./...     # exit 0
go test ./...      # all pass
go vet ./...       # clean
```

If any surviving test file pulls in a package that got deleted, the build fails and the deletion plan missed a touch point.

## 7. Documentation

### 7.1 CHANGELOG.md

Append under a new `### Breaking` block in the current unreleased section:

```markdown
### Breaking

- Removed the interactive TUI chat interface (`cli/ui/`) and the
  bubbletea-based config editor (`cli/ui/config/`, `cli/ui/webconfig/`).
  `hermind` and `hermind run` now launch the web UI and open the
  browser (equivalent to `hermind web`). Configuration lives in the
  Settings panel of the web UI — the standalone `hermind config`
  subcommand is removed. Headless usage:
  `hermind web --no-browser` plus an SSH tunnel to the bound port.
- Removed charmbracelet dependencies (bubbletea, bubbles, lipgloss,
  glamour). Downstream binaries gain ~4 MB of freed build size.
```

### 7.2 README.md

If a README exists at repo root:
- Rewrite any "Getting started" / "TUI" / "Interactive mode" section to describe the web UI flow.
- Update screenshots if any are committed (replace TUI screenshots with web screenshots).
- Update the "Commands" / "Usage" section to match §5.4.

If no README exists today, this section is a no-op.

### 7.3 Smoke doc

`docs/smoke/web-chat.md` (new, optional) — manual verification that `hermind` on a fresh clone boots the web server, opens the browser, lands on the chat workspace, and can complete one round trip. Can be deferred to after Phase 2 has its own smoke doc — don't double-write.

## 8. Rollback

The Phase 3 implementation lands as a single merge (or a small sequence). If something blocks the release:

1. `git revert <merge commit>` restores every deleted file and the old `go.mod` / `go.sum`.
2. Phase 1 (`sessionrun`, `session_registry`) and Phase 2 (React chat) are independent — the revert touches only Phase 3's footprint.
3. `go mod download` repopulates charmbracelet deps from `go.sum` on the reverted commit.

No destructive schema changes, no migrations to roll back.

## 9. Scope boundary checklist

Before marking this phase complete, `grep -rn`:

- `bubbletea` / `lipgloss` / `glamour` / `bubbles` — zero hits anywhere in the repo.
- `cli/ui` / `cli/ui/config` / `cli/ui/webconfig` / `cli/repl` / `cli/stub_provider` / `configui` / `webconfig` — zero hits in non-deleted files (git log will still have them; that's fine).
- `newConfigCmd` / `runREPL` — zero definitions or callers.

If any of these hit, deletion is incomplete — fix before merging.

## 10. Dependencies / ordering

- **Must merge after** Phase 1 AND Phase 2 are in `main`.
- The shared `cli/engine_deps.go` (Phase 1 deliverable) must exist; Phase 3 assumes it.
- Phase 2's chat workspace must actually work end-to-end — during Phase 3's plan execution, the smoke test for `hermind web` is the implicit gate.

If Phase 1 or Phase 2 slips, Phase 3 waits. Merging Phase 3 without Phase 2 leaves users with no chat interface at all.

## 11. Out of scope / future

- Adding a `--legacy-tui` escape hatch for users on no-browser terminals. If that need surfaces, it's a follow-up that resurrects a minimal chat TUI — not a Phase 3 rollback.
- Mobile-friendly web UI, dark mode, theme — Phase 2 follow-ups.
- Replacing any non-TUI CLI affordances (auth, setup, doctor stay CLI-only).
- Documentation migration to a dedicated docs site (still markdown in-repo).

## 12. Approval

After user review: invoke `writing-plans` skill to produce the implementation plan. Plan size estimate: 8-12 tasks (grep-verified deletions, single-file rewrites, dep cleanup, test migration, CHANGELOG).
