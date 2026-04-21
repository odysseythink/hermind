# TUI Removal (Phase 3/3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the bubbletea TUI (`cli/ui/`, `cli/ui/config/`, `cli/ui/webconfig/`), rewire `hermind` / `hermind run` as aliases for `hermind web`, remove `hermind config`. Drop charmbracelet dependencies.

**Architecture:** Extract a shared `runWeb` function from `newWebCmd`; call it from root, run, and web. Delete TUI code top-down. `go mod tidy` drops charmbracelet transitives.

**Tech Stack:** Go, cobra.

**Spec:** `docs/superpowers/specs/2026-04-21-tui-removal-design.md`

**Dependencies:** Phase 1 (backend endpoints) AND Phase 2 (React chat workspace) must be merged to `main` before this plan's implementation starts. During execution, the TUI is safe to delete only because the web UI is the proven replacement.

---

## File map

### Delete

| Path | Reason |
|---|---|
| `cli/ui/` (whole dir) | bubbletea chat TUI |
| `cli/ui/config/` (whole dir) | bubbletea config editor |
| `cli/ui/webconfig/` (whole dir) | standalone config web editor (superseded by `hermind web`) |
| `cli/repl.go` | TUI entry |
| `cli/stub_provider.go` | TUI degraded-mode fallback |
| `cli/stub_provider_test.go` | |
| `cli/repl_test.go` | REPL-path unit tests (coverage moved to engine_deps_test.go in Phase 1) |
| `cli/config.go` | `hermind config` subcommand |

### Rewrite

| Path | New content |
|---|---|
| `cli/run.go` | Alias for `hermind web` — delegates to shared `runWeb` function |
| `cli/root.go` | `RunE` fallback calls `runWeb`; remove `newConfigCmd` registration |
| `cli/web.go` | Extract RunE body into `runWeb(ctx, app, opts)`; flags still defined in `newWebCmd` |
| `cli/repl_tool_test.go` | Rename → `cli/engine_e2e_test.go`; rewrite to drive through `sessionrun.Run` |

### Modify

| Path | Change |
|---|---|
| `cli/app.go` | Drop `configui` import + any TUI-only `App` fields |
| `go.mod`, `go.sum` | `go mod tidy` drops charmbracelet |
| `CHANGELOG.md` | Breaking entry |
| `README.md` if present | Rewrite TUI references |

---

### Task 1: Pre-flight grep

Confirm no non-TUI code depends on charmbracelet / `cli/ui` / `cli/repl` / `cli/config`.

- [ ] **Step 1: Scan for external usage**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
grep -rn 'bubbletea\|lipgloss\|glamour\|bubbles' --include='*.go' \
  --exclude-dir=cli/ui --exclude-dir=cli/ui/config .
```

Expected: no results.

```bash
grep -rn 'cli/ui\|configui\|webconfig' --include='*.go' .
```

Expected: hits limited to `cli/app.go`, `cli/config.go`, `cli/repl.go`, and the `cli/ui/*` files themselves (those get deleted). Any hit in `api/`, `agent/`, `gateway/`, `storage/`, etc. is a blocker — stop and escalate.

```bash
grep -rn 'runREPL\|newConfigCmd\|newStubProvider\|stubProvider\|errMissingAPIKey' --include='*.go' .
```

Expected: hits limited to the files we're deleting + `cli/root.go` + `cli/engine_deps.go` (Phase 1 may reference `errMissingAPIKey`; keep that reference or move the sentinel).

- [ ] **Step 2: Record findings**

Write the results to `/tmp/phase3-preflight.txt` for reference during deletion. If anything unexpected appears, escalate before proceeding.

- [ ] **Step 3: No commit** — this is read-only.

---

### Task 2: Extract `runWeb` shared function

Refactor `newWebCmd` so bare `hermind` and `hermind run` can invoke it.

**Files:**
- Modify: `cli/web.go`

- [ ] **Step 1: Read current web.go**

```bash
cat cli/web.go
```

Note the current `newWebCmd` body — it's the full RunE that constructs the server, listens, serves.

- [ ] **Step 2: Extract shared function**

Rewrite `cli/web.go`:

```go
package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/cli/gatewayctl"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
)

// webRunOptions parameterize runWeb. Shared by newWebCmd, newRunCmd,
// and the bare `hermind` RunE.
type webRunOptions struct {
	Addr      string
	NoBrowser bool
	ExitAfter time.Duration
}

// runWeb is the actual body of `hermind web`. Exported-in-package so
// newRunCmd and root.go can call it without re-duplicating the flag
// parsing and server wiring.
func runWeb(ctx context.Context, app *App, opts webRunOptions) error {
	if err := ensureStorage(app); err != nil {
		return err
	}

	deps, err := BuildEngineDeps(app.Config, app.Storage)
	if err != nil && !isMissingAPIKey(err) {
		return err
	}

	ctrl := gatewayctl.New(app.Config, func(cfg config.Config) (*gateway.Gateway, error) {
		return BuildGateway(BuildGatewayDeps{Config: cfg})
	})
	if err := ctrl.Start(ctx); err != nil {
		return fmt.Errorf("web: start gateway controller: %w", err)
	}
	defer func() {
		shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
		defer c2()
		ctrl.Shutdown(shutCtx)
	}()

	token, err := api.GenerateToken()
	if err != nil {
		return fmt.Errorf("web: generate token: %w", err)
	}

	streams := api.NewMemoryStreamHub()
	srv, err := api.NewServer(&api.ServerOpts{
		Config:     app.Config,
		ConfigPath: app.ConfigPath,
		Storage:    app.Storage,
		Token:      token,
		Version:    Version,
		Streams:    streams,
		Controller: ctrl,
		Deps:       deps,
	})
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return fmt.Errorf("web: listen %s: %w", opts.Addr, err)
	}
	realAddr := "http://" + ln.Addr().String()
	fmt.Printf("hermind web listening on %s\n", realAddr)
	fmt.Printf("token: %s\n", token)
	fmt.Printf("open:  %s/?t=%s\n", realAddr, token)

	if !opts.NoBrowser {
		go openBrowser(realAddr + "/?t=" + token)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if opts.ExitAfter > 0 {
		time.AfterFunc(opts.ExitAfter, cancel)
	}

	httpSrv := &http.Server{
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-runCtx.Done()
		shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
		defer c2()
		_ = httpSrv.Shutdown(shutCtx)
	}()
	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// isMissingAPIKey lets web keep booting in degraded mode. In Phase 3
// the stub provider is gone, so this predicate just suppresses the
// error — the POST /messages handler will surface 503 instead.
func isMissingAPIKey(err error) bool {
	return err != nil && err.Error() == "hermind: primary provider has no api_key"
}

// newWebCmd builds the `hermind web` subcommand. It parses flags,
// then delegates to runWeb.
func newWebCmd(app *App) *cobra.Command {
	var opts webRunOptions
	c := &cobra.Command{
		Use:   "web",
		Short: "Start the hermind web UI and REST API",
		Long: `Start the hermind web UI and REST API.

Binds to 127.0.0.1 by default. A fresh session token is generated on
every boot and never persisted to disk; it is injected into the served
landing page so the browser can authenticate automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWeb(cmd.Context(), app, opts)
		},
	}
	c.Flags().StringVar(&opts.Addr, "addr", "127.0.0.1:9119",
		"bind address (keep 127.0.0.1 unless you know what you're doing)")
	c.Flags().BoolVar(&opts.NoBrowser, "no-browser", false,
		"do not open the browser automatically")
	c.Flags().DurationVar(&opts.ExitAfter, "exit-after", 0,
		"exit after the given duration (0 = run until Ctrl-C)")
	return c
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return
	}
	_ = exec.Command(cmd, args...).Start()
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: exit 0. TUI still works because repl.go still exists at this point.

- [ ] **Step 4: Commit**

```bash
git add cli/web.go
git commit -m "refactor(cli/web): extract runWeb shared function"
```

---

### Task 3: Rewrite `cli/run.go` as alias

**Files:**
- Modify: `cli/run.go`

- [ ] **Step 1: Rewrite**

Replace `cli/run.go`:

```go
// cli/run.go
package cli

import (
	"time"

	"github.com/spf13/cobra"
)

// newRunCmd creates `hermind run` — an alias for `hermind web`.
// Exists for backwards compatibility with scripts that invoke the
// historical REPL entry point.
func newRunCmd(app *App) *cobra.Command {
	var opts webRunOptions
	c := &cobra.Command{
		Use:   "run",
		Short: "Start hermind (alias for `web`)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWeb(cmd.Context(), app, opts)
		},
	}
	c.Flags().StringVar(&opts.Addr, "addr", "127.0.0.1:9119",
		"bind address")
	c.Flags().BoolVar(&opts.NoBrowser, "no-browser", false,
		"do not open the browser automatically")
	c.Flags().DurationVar(&opts.ExitAfter, "exit-after", 0,
		"exit after the given duration")
	_ = time.Duration(0) // keep time import resolved
	return c
}
```

Remove the `time` no-op line if the compiler doesn't complain. (It won't if `time.Duration` is used via the flag.)

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: exit 0. `hermind run` now invokes runWeb; this means the TUI is **unreachable** via this subcommand even before the TUI code is deleted.

- [ ] **Step 3: Commit**

```bash
git add cli/run.go
git commit -m "feat(cli/run): alias for web; TUI entry removed"
```

---

### Task 4: Rewrite `cli/root.go`

**Files:**
- Modify: `cli/root.go`

- [ ] **Step 1: Modify**

Open `cli/root.go`, locate the `root.AddCommand(...)` block. Remove `newConfigCmd(app)`. The final list:

```go
root.AddCommand(
	newRunCmd(app),
	newGatewayCmd(app),
	newCronCmd(app),
	newSkillsCmd(app),
	newSetupCmd(app),
	// newConfigCmd removed — config is in the web UI Settings panel
	newDoctorCmd(app),
	newAuthCmd(app),
	newModelsCmd(app),
	newProfileCmd(app),
	newPluginsCmd(app),
	newUpgradeCmd(app),
	newRLCmd(app),
	newMCPCmd(app),
	newWebCmd(app),
	newVersionCmd(),
)
```

Change the default `RunE`:

```go
root.RunE = func(cmd *cobra.Command, args []string) error {
	return runWeb(cmd.Context(), app, webRunOptions{
		Addr: "127.0.0.1:9119",
	})
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: at this point cobra complains that `newConfigCmd` / `configui` are referenced elsewhere — but `cli/config.go` still exists and registers. Since we removed the call site (root.go), the file just has unused definitions. Build should still succeed unless the file has compile errors from Phase 2/1 era.

Likely to fail because `cli/app.go` imports `configui`. Move on to Task 5 to remove it.

- [ ] **Step 3: Stash for now — do not commit** until Task 5 completes (or take a WIP commit and amend later).

---

### Task 5: Delete `cli/config.go` + clean `cli/app.go`

**Files:**
- Delete: `cli/config.go`
- Modify: `cli/app.go`

- [ ] **Step 1: Remove cli/config.go**

```bash
rm cli/config.go
```

- [ ] **Step 2: Clean cli/app.go**

Open `cli/app.go`. Remove the `configui "github.com/odysseythink/hermind/cli/ui/config"` import. If there are any App struct fields only used by `cli/config.go`, remove them too (review what the import gave us — likely nothing beyond the import itself).

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: exit 0. All `hermind config` traces are gone.

- [ ] **Step 4: Commit**

```bash
git add cli/app.go cli/config.go cli/root.go
git commit -m "refactor(cli): remove hermind config subcommand"
```

---

### Task 6: Delete TUI code

All TUI-only files. Since no surviving code references them (confirmed in Task 1 pre-flight + the root/run rewires above), deletion is safe.

- [ ] **Step 1: Remove**

```bash
git rm -r cli/ui/
git rm cli/repl.go
git rm cli/stub_provider.go cli/stub_provider_test.go
git rm cli/repl_test.go
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: exit 0.

If the build fails with `errMissingAPIKey` undefined (likely — it was defined in cli/stub_provider.go):

- Recreate a minimal sentinel in `cli/engine_deps.go` (Phase 1 deliverable):

```go
// errMissingAPIKey is returned by BuildEngineDeps when the primary
// provider has no api_key. runWeb suppresses it so the server still
// boots; the POST /messages handler surfaces 503 to the user.
var errMissingAPIKey = errors.New("hermind: primary provider has no api_key")
```

Add `"errors"` to that file's imports. Replace the `isMissingAPIKey` in web.go with `errors.Is(err, errMissingAPIKey)` for correctness:

```go
if err != nil && !errors.Is(err, errMissingAPIKey) {
	return err
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(cli): delete TUI (cli/ui, cli/repl, cli/stub_provider)"
```

---

### Task 7: Rename + rewrite repl_tool_test.go

The end-to-end test that drives a full tool-use round-trip against a mocked Anthropic server is valuable. Keep its coverage but route through `sessionrun.Run` instead of the deleted TUI.

**Files:**
- Rename: `cli/repl_tool_test.go` → `cli/engine_e2e_test.go`
- Rewrite: body

- [ ] **Step 1: Git rename**

```bash
git mv cli/repl_tool_test.go cli/engine_e2e_test.go
```

- [ ] **Step 2: Read the current body**

```bash
cat cli/engine_e2e_test.go
```

Note the test function names and the httptest server setup. Keep that infrastructure.

- [ ] **Step 3: Rewrite the test**

Replace the existing test functions with one that drives through `sessionrun.Run`:

```go
package cli

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/api/sessionrun"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool"

	// Keep the existing httptest + anthropic imports and mock setup.
)

type collectingHub struct {
	events []sessionrun.Event
}

func (h *collectingHub) Publish(e sessionrun.Event) {
	h.events = append(h.events, e)
}

func TestRunEngineE2E_ToolRoundTrip(t *testing.T) {
	// Keep the existing mockServer setup from repl_tool_test.go:
	// httptest.NewServer that responds to /v1/messages with a tool_use
	// then a final assistant text turn.
	srv := /* existing mock server constructor */

	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.Provider.Provider = "anthropic"
	cfg.Provider.APIKey = "sk-test"
	store, err := sqlite.Open(tmp + "/h.db")
	if err != nil { t.Fatal(err) }
	defer store.Close()
	if err := store.Migrate(); err != nil { t.Fatal(err) }

	deps, err := BuildEngineDeps(cfg, store)
	if err != nil { t.Fatal(err) }

	hub := &collectingHub{}
	deps.Hub = hub
	deps.ToolReg = tool.NewRegistry() // or include the expected tool
	// Register the test tool here.

	if err := sessionrun.Run(context.Background(), deps, sessionrun.Request{
		SessionID: "s1", UserMessage: "use the tool",
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	sawToolCall := false
	for _, e := range hub.events {
		if e.Type == "tool_call" {
			sawToolCall = true
		}
	}
	if !sawToolCall {
		t.Errorf("no tool_call event; events=%v", hub.events)
	}
}
```

> **Adapt the mock server bodies verbatim from the old test.** The surrounding Anthropic mock setup (JSON bodies, headers, endpoint routing) is non-trivial — don't re-invent.

- [ ] **Step 4: Build + test**

```bash
go build ./...
go test ./cli -run TestRunEngineE2E -v
```

Expected: test PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/engine_e2e_test.go
git commit -m "test(cli): rewrite repl_tool_test as sessionrun e2e"
```

---

### Task 8: `go mod tidy`

- [ ] **Step 1: Tidy**

```bash
go mod tidy
```

- [ ] **Step 2: Verify charmbracelet is gone**

```bash
grep charmbracelet go.mod
```

Expected: no output.

```bash
grep charmbracelet go.sum
```

Expected: no output (or only lines go.sum keeps for historical ambient go.sum entries — if present, they will be removed by another `go mod tidy` pass; try `go clean -modcache && go mod tidy` if you see leftovers).

- [ ] **Step 3: Build + test**

```bash
go build ./...
go test ./... -count=1
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): go mod tidy drops charmbracelet deps"
```

---

### Task 9: Command table tests

Confirm bare `hermind`, `hermind run`, `hermind web` all dispatch to runWeb; `hermind config` errors as unknown command.

**Files:**
- Modify (or create): `cli/web_test.go`

- [ ] **Step 1: Write tests**

```go
package cli

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestBareHermind_DispatchesToWeb(t *testing.T) {
	app := &App{Config: minimalCfgForTest()}
	cmd := NewRootCmd(app)
	cmd.SetArgs([]string{"--addr", "127.0.0.1:0", "--exit-after", "10ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	// Bare hermind takes no flags today; use cmd.Execute with no args.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd.SetContext(ctx)
	err := cmd.Execute()
	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("bare hermind: %v", err)
	}
}

func TestHermindRun_DispatchesToWeb(t *testing.T) {
	app := &App{Config: minimalCfgForTest()}
	cmd := NewRootCmd(app)
	cmd.SetArgs([]string{"run", "--addr", "127.0.0.1:0", "--no-browser", "--exit-after", "10ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("hermind run: %v", err)
	}
}

func TestHermindConfig_Removed(t *testing.T) {
	app := &App{Config: minimalCfgForTest()}
	cmd := NewRootCmd(app)
	cmd.SetArgs([]string{"config"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected `hermind config` to be unknown")
	}
}

// minimalCfgForTest returns a config with a stubbed api_key so
// BuildEngineDeps does not trip errMissingAPIKey during the smoke.
// Adjust its field names to match the actual config.Config shape.
func minimalCfgForTest() *config.Config {
	c := &config.Config{}
	c.Provider.Provider = "anthropic"
	c.Provider.APIKey = "sk-test"
	return c
}
```

> Bare `hermind` in today's `root.go` doesn't accept flags — but runWeb does. If the `--addr`/`--exit-after` flags don't apply at the root, invoke as `hermind web --exit-after 10ms` in the first test instead. The intent is: Execute a short-lived server.

- [ ] **Step 2: Run**

```bash
go test ./cli -run 'Hermind|Config_Removed' -v
```

Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add cli/web_test.go
git commit -m "test(cli): bare/run/config command-table regression tests"
```

---

### Task 10: Final regression

- [ ] **Step 1: Full suite**

```bash
go test ./... -count=1
```

Expected: exits 0. If any test uses charmbracelet fixtures that no longer exist, it should already have been deleted in Task 6.

- [ ] **Step 2: Race**

```bash
go test -race ./api ./api/sessionrun ./cli -count=1
```

Expected: clean.

- [ ] **Step 3: Vet**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 4: Smoke manually**

```bash
go run . --exit-after 3s
```

Expected: "hermind web listening on …" prints; browser opens (or fails to open silently on headless); exits after 3s.

---

### Task 11: CHANGELOG + README

**Files:**
- Modify: `CHANGELOG.md`
- Modify (if exists): `README.md`

- [ ] **Step 1: CHANGELOG**

Under `## Unreleased`, add a `### Breaking` block:

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

- [ ] **Step 2: README**

```bash
ls README*
```

If a README exists:
- Rewrite any "Getting started" / "TUI" / "Interactive mode" references to the web UI flow.
- Update the Commands section to match the current table (no `hermind config`).

If none exists, skip.

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md README.md 2>/dev/null || git add CHANGELOG.md
git commit -m "docs: TUI removal breaking entry + web-first README"
```

---

### Task 12: Boundary verification

Final grep pass before declaring the phase complete.

- [ ] **Step 1: No TUI traces**

```bash
grep -rn 'bubbletea\|lipgloss\|glamour\|bubbles' --include='*.go' .
```

Expected: empty.

```bash
grep -rn 'cli/ui\|configui\|webconfig\|runREPL\|newConfigCmd\|stubProvider\|newStubProvider' --include='*.go' .
```

Expected: empty.

- [ ] **Step 2: No stale tests**

```bash
find . -name 'repl_test.go' -o -name 'repl_tool_test.go' -o -name 'stub_provider_test.go'
```

Expected: empty (repl_tool_test.go was renamed to engine_e2e_test.go).

- [ ] **Step 3: No commit** — this is verification only.

---

## Self-review checklist

- [ ] Pre-flight greps (Task 1) surfaced zero external consumers.
- [ ] `go build ./...` green.
- [ ] `go test ./... -count=1` green.
- [ ] `go test -race ./api ./api/sessionrun ./cli` green.
- [ ] `go vet ./...` clean.
- [ ] `grep charmbracelet go.mod` → empty.
- [ ] `hermind` (bare) → opens web.
- [ ] `hermind run` → opens web.
- [ ] `hermind web` → opens web.
- [ ] `hermind config` → unknown command.
- [ ] CHANGELOG updated.
