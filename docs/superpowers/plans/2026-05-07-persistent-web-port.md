# Persistent Web Server Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the web server bind address persistent across restarts by writing the chosen port to config.yaml on first start and re-reading it on subsequent starts.

**Architecture:** Add an `Addr` field to `config.WebConfig`, extract the listener-binding logic from `runWeb()` into a `bindListener()` helper that checks CLI flag → config → random port (with persistence), and update `hermind run` to default to empty `--addr`.

**Tech Stack:** Go, Cobra, yaml.v3, net.Listen, log/slog

---

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `config/config.go` | Modify | Add `Addr string` to `WebConfig` struct |
| `config/config_test.go` | Modify | Add YAML round-trip test for `WebConfig.Addr` |
| `cli/web.go` | Modify | Add `bindListener()` helper; replace inline binding in `runWeb()` |
| `cli/run.go` | Modify | Change `--addr` default from `"127.0.0.1:9119"` to `""` |
| `cli/web_test.go` | Modify | Add unit tests for `bindListener()` |

---

### Task 1: Add `Addr` to `WebConfig` + YAML Test

**Files:**
- Modify: `config/config.go:38-41`
- Test: `config/config_test.go`

- [ ] **Step 1: Add `Addr` field**

In `config/config.go`, add `Addr` to `WebConfig`:

```go
type WebConfig struct {
	Search          SearchConfig `yaml:"search,omitempty"`
	DisableWebFetch bool         `yaml:"disable_web_fetch,omitempty"`
	Addr            string       `yaml:"addr,omitempty"` // NEW
}
```

- [ ] **Step 2: Write YAML round-trip test**

Append to `config/config_test.go`:

```go
func TestWebConfigYAMLRoundTrip(t *testing.T) {
	yamlSrc := []byte(
		"web:\n" +
			"  addr: 127.0.0.1:34567\n" +
			"  disable_web_fetch: true\n",
	)
	var cfg Config
	require.NoError(t, yaml.Unmarshal(yamlSrc, &cfg))
	require.Equal(t, "127.0.0.1:34567", cfg.Web.Addr)
	require.True(t, cfg.Web.DisableWebFetch)
}
```

- [ ] **Step 3: Run the new test**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./config -run TestWebConfigYAMLRoundTrip -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "feat(config): add Web.Addr field for persistent bind address"
```

---

### Task 2: Extract `bindListener` and Wire It Into `runWeb`

**Files:**
- Modify: `cli/web.go`

- [ ] **Step 1: Add imports**

Update the import block in `cli/web.go` to include `log/slog` and `config`:

```go
import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/agent/idle"
	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
)
```

- [ ] **Step 2: Add `bindListener` helper**

Insert the following function directly above `runWeb` in `cli/web.go`:

```go
// bindListener resolves the TCP listener for the web server.
// Priority:
//   1. opts.Addr (CLI flag) — not persisted
//   2. app.Config.Web.Addr (config file) — used as-is
//   3. random port in [30000,40000) — persisted to config on success
//
// If the configured address fails to bind (e.g. port already in use),
// it is cleared and a random port is chosen instead.
func bindListener(app *App, opts webRunOptions) (net.Listener, error) {
	// 1. CLI override — highest priority, never persist
	if opts.Addr != "" {
		ln, err := net.Listen("tcp", opts.Addr)
		if err != nil {
			return nil, fmt.Errorf("web: listen %s: %w", opts.Addr, err)
		}
		return ln, nil
	}

	// 2. Previously persisted address
	if app.Config.Web.Addr != "" {
		ln, err := net.Listen("tcp", app.Config.Web.Addr)
		if err == nil {
			return ln, nil
		}
		// Stale or invalid address — clear and fall through to random
		app.Config.Web.Addr = ""
	}

	// 3. First start or stale address — random port
	ln, err := listenRandomLocalhost()
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}

	// Persist the chosen address
	app.Config.Web.Addr = ln.Addr().String()
	if err := config.SaveToPath(app.ConfigPath, app.Config); err != nil {
		slog.Warn("web: failed to persist bind address", "error", err)
	}

	return ln, nil
}
```

- [ ] **Step 3: Replace inline binding in `runWeb`**

In `cli/web.go`, replace the existing block (currently lines 61-72):

```go
	var ln net.Listener
	if opts.Addr == "" {
		ln, err = listenRandomLocalhost()
		if err != nil {
			return fmt.Errorf("web: %w", err)
		}
	} else {
		ln, err = net.Listen("tcp", opts.Addr)
		if err != nil {
			return fmt.Errorf("web: listen %s: %w", opts.Addr, err)
		}
	}
```

With:

```go
	ln, err := bindListener(app, opts)
	if err != nil {
		return err
	}
```

- [ ] **Step 4: Verify compilation**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go build ./cli/...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add cli/web.go
git commit -m "feat(cli): bindListener helper with config persistence"
```

---

### Task 3: Update `hermind run` Default `--addr`

**Files:**
- Modify: `cli/run.go:21`

- [ ] **Step 1: Change the default flag value**

In `cli/run.go`, change:

```go
	c.Flags().StringVar(&opts.Addr, "addr", "127.0.0.1:9119", "bind address")
```

To:

```go
	c.Flags().StringVar(&opts.Addr, "addr", "", "bind address; empty = read from config or random port")
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go build ./cli/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add cli/run.go
git commit -m "feat(cli): run command defaults to empty addr for config-driven binding"
```

---

### Task 4: Unit Tests for `bindListener`

**Files:**
- Modify: `cli/web_test.go`

- [ ] **Step 1: Add imports**

Add `"net"` and `"github.com/odysseythink/hermind/config"` to the import block in `cli/web_test.go`:

```go
import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Write `bindListener` tests**

Append the following tests to `cli/web_test.go`:

```go
func TestBindListener_CliAddrNotPersisted(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.Default()
	require.NoError(t, config.SaveToPath(cfgPath, cfg))

	app := &App{Config: cfg, ConfigPath: cfgPath}

	ln, err := bindListener(app, webRunOptions{Addr: "127.0.0.1:0"})
	require.NoError(t, err)
	defer ln.Close()

	// CLI address should NOT be written to config
	assert.Empty(t, app.Config.Web.Addr)

	loaded, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Empty(t, loaded.Web.Addr)
}

func TestBindListener_PersistsRandomPort(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.Default()
	require.NoError(t, config.SaveToPath(cfgPath, cfg))

	app := &App{Config: cfg, ConfigPath: cfgPath}

	ln, err := bindListener(app, webRunOptions{})
	require.NoError(t, err)
	defer ln.Close()

	assert.NotEmpty(t, app.Config.Web.Addr)
	assert.Contains(t, app.Config.Web.Addr, "127.0.0.1:")

	loaded, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, app.Config.Web.Addr, loaded.Web.Addr)
}

func TestBindListener_ReusesConfiguredAddr(t *testing.T) {
	// Grab a free port, close it, then configure it
	tmpLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := tmpLn.Addr().String()
	tmpLn.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.Default()
	cfg.Web.Addr = addr
	require.NoError(t, config.SaveToPath(cfgPath, cfg))

	app := &App{Config: cfg, ConfigPath: cfgPath}

	ln, err := bindListener(app, webRunOptions{})
	require.NoError(t, err)
	defer ln.Close()

	assert.Equal(t, addr, app.Config.Web.Addr)
}

func TestBindListener_FallsBackWhenConfiguredAddrInUse(t *testing.T) {
	occupier, err := net.Listen("tcp", "127.0.0.1:35001")
	require.NoError(t, err)
	defer occupier.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.Default()
	cfg.Web.Addr = "127.0.0.1:35001"
	require.NoError(t, config.SaveToPath(cfgPath, cfg))

	app := &App{Config: cfg, ConfigPath: cfgPath}

	ln, err := bindListener(app, webRunOptions{})
	require.NoError(t, err)
	defer ln.Close()

	assert.NotEqual(t, "127.0.0.1:35001", app.Config.Web.Addr)
	assert.NotEmpty(t, app.Config.Web.Addr)

	loaded, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.NotEqual(t, "127.0.0.1:35001", loaded.Web.Addr)
}
```

- [ ] **Step 3: Run the new tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./cli -run TestBindListener -v
```

Expected: all 4 PASS

- [ ] **Step 4: Commit**

```bash
git add cli/web_test.go
git commit -m "test(cli): unit tests for bindListener persistence logic"
```

---

### Task 5: Full Test Suite + Final Commit

- [ ] **Step 1: Run all tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./... -count=1
```

Expected: all PASS (should already be passing; we only added new tests)

- [ ] **Step 2: Build binary**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go build ./cmd/hermind
```

Expected: no errors

- [ ] **Step 3: Final commit**

```bash
git commit --allow-empty -m "feat: persistent web server port across restarts"
```

---

## Self-Review

**1. Spec coverage:**
- ✅ Config field `Web.Addr` — Task 1
- ✅ First start randomizes and persists — Task 2 (`bindListener` random path + `SaveToPath`)
- ✅ Subsequent starts reuse config — Task 2 (`bindListener` config path)
- ✅ CLI `--addr` takes priority and is not persisted — Task 2 + Task 3
- ✅ `hermind run` no longer hardcodes 9119 — Task 3
- ✅ Configured addr in use falls back to random — Task 2 + Task 4 test
- ✅ Save failure is non-fatal — Task 2 (`slog.Warn`, no return)

**2. Placeholder scan:**
- No TBD/TODO/"implement later"/"fill in details"
- No vague "add error handling" — specific `slog.Warn` call shown
- No "Similar to Task N" — each task is self-contained
- All code blocks contain the actual code to write

**3. Type consistency:**
- `config.Web.Addr` is `string` everywhere
- `bindListener` signature matches usage in `runWeb`
- `config.SaveToPath` signature matches existing code
- `isAddrInUse` is used implicitly (same package, no signature change)

---

**Plan complete and saved to `docs/superpowers/plans/2026-05-07-persistent-web-port.md`.**

Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
