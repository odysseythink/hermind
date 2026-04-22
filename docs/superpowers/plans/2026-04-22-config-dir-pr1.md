# PR 1: config-dir — Instance-Bound Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every hermind process a self-contained instance rooted at cwd, with independent config/state/skills/trajectories, random localhost port, no token auth, and a frontend that displays the instance absolute path prominently.

**Architecture:** Single entry point `config.InstanceRoot()` returns `$HERMIND_HOME` if set, else `./.hermind/`. Every derived path (`config.yaml`, `state.db`, `skills/`, `trajectories/`, `enabled_skills.yaml`, `credentials.yaml`, `plugins/`) reads from that root. No home-directory fallback. Token-based auth is removed — localhost-only binding is the sole access control. Port is randomized in `[30000, 40000)` with retry on conflict.

**Tech Stack:** Go 1.25 (math/rand/v2), chi router, testify for Go tests; React + Vite + Vitest for frontend; pnpm for frontend build.

**Spec reference:** `docs/superpowers/specs/2026-04-22-hermind-instance-single-session-design.md` §PR 1.

---

## File Structure

**New files:**
- `config/instance.go` — `InstanceRoot()` entry point; ~30 lines.
- `config/instance_test.go` — unit tests for `InstanceRoot()`.
- `cli/listen.go` — `listenRandomLocalhost()` helper; ~40 lines.
- `cli/listen_test.go` — unit tests (port reuse retry, all-ports-busy failure).

**Modified files:**
- `config/loader.go` — drop all `~/.hermind` fallbacks; `resolveDefaults` uses `InstanceRoot`.
- `cli/app.go` — `defaultConfigPath` → `InstanceRoot`; first-run adds `.migration_notice_shown` marker.
- `cli/app_test.go` — replace `HERMIND_HOME` setenv with `t.Chdir(tmp)` where appropriate; keep `HERMIND_HOME` test case as "override" path.
- `cli/bootstrap.go` — `ensureStorage` resolves default SQLite path via `InstanceRoot`; update error-message path hint.
- `cli/plugins.go` — `enabledSkillsPath()` uses `InstanceRoot`, drops profile layer.
- `cli/models.go` — `models switch` writes to `InstanceRoot/config.yaml`.
- `cli/auth.go` — `defaultCredentialsPath` uses `InstanceRoot`.
- `cli/setup.go` — writes config.yaml to `InstanceRoot` (not `~/.hermind`).
- `cli/doctor.go` — default SQLite path uses `InstanceRoot`.
- `cli/skills.go` — uses `InstanceRoot` for any skill path defaults (audit).
- `cli/root.go` — remove `newProfileCmd(app)` from command tree; change default `root.RunE` to pass `Addr: ""` (empty = random port).
- `cli/web.go` — delete token generation/printing; call `listenRandomLocalhost()` when `opts.Addr` empty; new banner.
- `agent/trajectory.go` — `DefaultTrajectoryDir` uses `InstanceRoot`.
- `agent/prompt.go` — doc string refresh (`$HERMIND_HOME/skills` → `<instance>/skills`).
- `api/server.go` — drop `Token` field from `ServerOpts`; drop auth middleware wiring; drop `{{TOKEN}}` injection in `handleIndex`; accept `InstanceRoot` string.
- `api/dto.go` — add `InstanceRoot string json:"instance_root"` field to `StatusResponse`.
- `api/handlers_meta.go` — populate `InstanceRoot` in `handleStatus`.
- `api/server_test.go` — remove Token test inputs; update to new ServerOpts shape.
- `web/index.html` — drop `window.HERMIND.token` inline script.
- `web/src/main.tsx` — fetch `/api/status`, set `document.title = "hermind — <last-segment>"`; pass `instanceRoot` into App via a simple top-level state.
- `web/src/App.tsx` — accept or lift `instanceRoot` state; forward to `ConversationHeader` via `ChatWorkspace`.
- `web/src/components/chat/ConversationHeader.tsx` — add instance path label (left, monospace, `dir="rtl"`, `title` tooltip).
- `web/src/components/chat/ConversationHeader.module.css` — styling for the label.
- `web/src/api/client.ts` — delete `resolveToken`, drop `Authorization` header.
- `web/src/hooks/useChatStream.ts` — drop `?t=` query param from WS/SSE URLs.
- `CHANGELOG.md` — entry describing cwd-rooted config + random port + token removal.
- `api/webroot/` — rebuilt bundle at the end.

**Deleted files:**
- `cli/profile.go` — profile concept removed entirely.
- `api/auth.go` — token middleware removed.
- `api/auth_test.go` — corresponding tests removed.

---

## Task 1: `config.InstanceRoot()` helper with TDD

**Files:**
- Create: `config/instance.go`
- Create: `config/instance_test.go`

- [ ] **Step 1: Write the failing test**

Create `config/instance_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceRoot_CwdDefault(t *testing.T) {
	// No HERMIND_HOME set → root = cwd/.hermind
	t.Setenv("HERMIND_HOME", "")
	tmp := t.TempDir()
	t.Chdir(tmp)

	got, err := InstanceRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, ".hermind"), got)
}

func TestInstanceRoot_HermindHomeOverride(t *testing.T) {
	// HERMIND_HOME set → root = $HERMIND_HOME verbatim
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", tmp)

	got, err := InstanceRoot()
	require.NoError(t, err)
	assert.Equal(t, tmp, got)
}

func TestInstanceRoot_HermindHomeTrimsWhitespace(t *testing.T) {
	// Whitespace-only HERMIND_HOME behaves as unset.
	t.Setenv("HERMIND_HOME", "   ")
	tmp := t.TempDir()
	t.Chdir(tmp)

	got, err := InstanceRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, ".hermind"), got)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/... -run TestInstanceRoot -v`
Expected: FAIL with `undefined: InstanceRoot`.

- [ ] **Step 3: Write the implementation**

Create `config/instance.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstanceRoot returns the absolute path to this hermind instance's
// root directory. Resolution order:
//
//  1. $HERMIND_HOME (honored verbatim — caller decides whether it
//     already ends in ".hermind" or not).
//  2. <cwd>/.hermind
//
// There is no home-directory fallback. Each working directory is its
// own hermind instance.
func InstanceRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("HERMIND_HOME")); v != "" {
		return v, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("config: resolve cwd: %w", err)
	}
	return filepath.Join(cwd, ".hermind"), nil
}

// InstancePath joins one or more path components onto the instance root.
// Returns the error from InstanceRoot on failure.
func InstancePath(parts ...string) (string, error) {
	root, err := InstanceRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{root}, parts...)...), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/... -run TestInstanceRoot -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/instance.go config/instance_test.go
git commit -m "feat(config): introduce InstanceRoot (cwd-rooted, HERMIND_HOME override)"
```

---

## Task 2: `config.Load` + `resolveDefaults` use `InstanceRoot`

**Files:**
- Modify: `config/loader.go`
- Modify: `config/loader_test.go`

- [ ] **Step 1: Write the failing test**

Append to `config/loader_test.go`:

```go
func TestLoad_UsesInstanceRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", tmp)

	// Write a minimal config at <root>/config.yaml
	cfgPath := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("model: anthropic/claude-sonnet-4-6\n"), 0o644))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", cfg.Model)
}

func TestResolveDefaults_SQLitePathUsesInstanceRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", tmp)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "state.db"), cfg.Storage.SQLitePath)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./config/... -run "TestLoad_UsesInstanceRoot|TestResolveDefaults_SQLitePathUsesInstanceRoot" -v`
Expected: FAIL — old `Load` hits `~/.hermind/config.yaml`; `resolveDefaults` uses home dir.

- [ ] **Step 3: Rewrite `config/loader.go`**

Replace the file contents with:

```go
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultConfigFile is the file name inside the instance root.
const (
	DefaultConfigFile = "config.yaml"
	DefaultDBFile     = "state.db"
)

// Load reads <instance-root>/config.yaml. Missing file returns defaults.
func Load() (*Config, error) {
	root, err := InstanceRoot()
	if err != nil {
		return nil, err
	}
	return LoadFromPath(filepath.Join(root, DefaultConfigFile))
}

// LoadFromPath reads a specific config file. Missing file returns defaults.
func LoadFromPath(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		resolveDefaults(cfg)
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	resolveDefaults(cfg)
	return cfg, nil
}

// resolveDefaults fills in environment-dependent values after a load.
func resolveDefaults(cfg *Config) {
	if cfg.Storage.SQLitePath == "" {
		if root, err := InstanceRoot(); err == nil {
			cfg.Storage.SQLitePath = filepath.Join(root, DefaultDBFile)
		}
	}
}
```

Note: the old `DefaultConfigDir = "~/.hermind"` constant is removed. The old `expandPath` helper is also removed; nothing in the new code needs to expand `~`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./config/... -v`
Expected: all tests pass, including the new ones.

- [ ] **Step 5: Commit**

```bash
git add config/loader.go config/loader_test.go
git commit -m "refactor(config): Load/resolveDefaults use InstanceRoot (drop ~/.hermind)"
```

---

## Task 3: `cli/app.go:NewApp` uses `InstanceRoot` + first-run migration notice

**Files:**
- Modify: `cli/app.go`
- Modify: `cli/app_test.go`

- [ ] **Step 1: Replace `cli/app_test.go` contents**

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApp_HermindHomeOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("model: anthropic/claude-opus-4-6\n"), 0o644))

	app, err := NewApp()
	require.NoError(t, err)
	defer app.Close()

	assert.Equal(t, cfgPath, app.ConfigPath)
}

func TestNewApp_CwdFirstRunWritesDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", "")
	t.Chdir(tmp)

	app, err := NewApp()
	require.NoError(t, err)
	defer app.Close()

	assert.Equal(t, filepath.Join(tmp, ".hermind", "config.yaml"), app.ConfigPath)

	_, err = os.Stat(filepath.Join(tmp, ".hermind", "config.yaml"))
	assert.NoError(t, err, "first-run should write default config.yaml")
}

func TestNewApp_MigrationNoticeFiresOnceWhenHomeHermindExists(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", "")
	t.Chdir(tmp)

	// Simulate a legacy ~/.hermind via HOME override.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	require.NoError(t, os.MkdirAll(filepath.Join(fakeHome, ".hermind"), 0o755))

	// First boot: marker does not exist.
	marker := filepath.Join(tmp, ".hermind", ".migration_notice_shown")
	_, err := os.Stat(marker)
	require.True(t, os.IsNotExist(err))

	app, err := NewApp()
	require.NoError(t, err)
	app.Close()

	// Marker must now exist.
	_, err = os.Stat(marker)
	assert.NoError(t, err, "first boot should create .migration_notice_shown marker")

	// Second boot: marker exists, so NewApp must not re-create the entire instance.
	app2, err := NewApp()
	require.NoError(t, err)
	app2.Close()
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli/... -run "TestNewApp_" -v`
Expected: FAIL on `TestNewApp_CwdFirstRunWritesDefault` and `TestNewApp_MigrationNoticeFiresOnceWhenHomeHermindExists`.

- [ ] **Step 3: Rewrite `cli/app.go`**

```go
// cli/app.go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

type App struct {
	Config       *config.Config
	ConfigPath   string
	InstanceRoot string
	Storage      storage.Storage
}

// NewApp loads the instance config. First-run behavior:
//   - creates <instance-root>/ if missing
//   - writes a default config.yaml if missing
//   - if ~/.hermind exists and HERMIND_HOME is unset and the instance
//     has not shown the notice yet, prints a one-time stderr hint and
//     touches a marker file so the hint does not repeat.
func NewApp() (*App, error) {
	root, err := config.InstanceRoot()
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(root, config.DefaultConfigFile)

	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("hermind: create instance root %s: %w", root, err)
	}

	if _, statErr := os.Stat(cfgPath); errors.Is(statErr, os.ErrNotExist) {
		if err := writeDefaultConfig(cfgPath); err != nil {
			return nil, fmt.Errorf("write default config: %w", err)
		}
		fmt.Fprintf(os.Stderr,
			"hermind: wrote default config to %s — run `hermind web` to configure providers.\n",
			cfgPath,
		)
	}

	maybePrintMigrationNotice(root)

	cfg, err := config.LoadFromPath(cfgPath)
	if err != nil {
		return nil, err
	}
	return &App{
		Config:       cfg,
		ConfigPath:   cfgPath,
		InstanceRoot: root,
	}, nil
}

func (a *App) Close() error {
	if a.Storage != nil {
		return a.Storage.Close()
	}
	return nil
}

// maybePrintMigrationNotice emits a one-time stderr hint when the user
// has a legacy ~/.hermind/ directory but is now running under a new
// cwd-rooted instance. Respects $HERMIND_HOME (no notice when caller
// has explicitly chosen a root).
func maybePrintMigrationNotice(root string) {
	if os.Getenv("HERMIND_HOME") != "" {
		return
	}
	marker := filepath.Join(root, ".migration_notice_shown")
	if _, err := os.Stat(marker); err == nil {
		return // already shown
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	legacy := filepath.Join(home, ".hermind")
	// Don't show the notice if the cwd instance *is* ~/.hermind.
	if absRoot, errA := filepath.Abs(root); errA == nil {
		if absLegacy, errL := filepath.Abs(legacy); errL == nil && absRoot == absLegacy {
			_ = os.WriteFile(marker, []byte("same-as-legacy\n"), 0o644)
			return
		}
	}
	if _, err := os.Stat(legacy); err != nil {
		return // no legacy directory → nothing to notice
	}
	fmt.Fprintln(os.Stderr,
		"hermind: legacy config at ~/.hermind/ is not auto-inherited by this instance.")
	fmt.Fprintln(os.Stderr,
		"  If you want to reuse it, copy manually: cp -r ~/.hermind/. "+root+"/")
	_ = os.WriteFile(marker, []byte("shown\n"), 0o644)
}

func writeDefaultConfig(path string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	cfg := config.Default()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
```

The old `defaultConfigPath` function is removed — callers migrate to `app.InstanceRoot` or `app.ConfigPath`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./cli/... -run "TestNewApp_" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/app.go cli/app_test.go
git commit -m "refactor(cli): NewApp uses InstanceRoot + one-time migration notice"
```

---

## Task 4: Remove the profile concept

**Files:**
- Delete: `cli/profile.go`
- Modify: `cli/plugins.go`
- Modify: `cli/root.go`

- [ ] **Step 1: Delete `cli/profile.go`**

```bash
git rm cli/profile.go
```

- [ ] **Step 2: Remove `newProfileCmd` from the command tree**

Edit `cli/root.go` — remove the `newProfileCmd(app),` line from the `AddCommand(...)` call.

- [ ] **Step 3: Rewrite `cli/plugins.go:enabledSkillsPath` and `profileLabel`**

Replace the `enabledSkillsPath()` function and the `profileLabel()` function in `cli/plugins.go`:

```go
// enabledSkillsPath returns <instance-root>/enabled_skills.yaml.
func enabledSkillsPath() string {
	p, err := config.InstancePath("enabled_skills.yaml")
	if err != nil {
		// InstanceRoot only fails if cwd lookup fails — extremely rare.
		// Fall back to a relative path; any write will error out at the
		// caller and surface the real problem.
		return ".hermind/enabled_skills.yaml"
	}
	return p
}
```

Remove the `profileLabel()` function entirely. Update the two call sites in `newPluginsEnableCmd` and `newPluginsDisableCmd` that used `profileLabel()` — replace those `fmt.Fprintf` lines with:

```go
fmt.Fprintf(cmd.OutOrStdout(), "enabled %s\n", args[0])
```
and
```go
fmt.Fprintf(cmd.OutOrStdout(), "disabled %s\n", args[0])
```

Add `"github.com/odysseythink/hermind/config"` to the imports and remove the `"os"`, `"path/filepath"` imports if they are no longer needed (grep the file after editing — likely still needed for `os.MkdirAll` / `filepath.Dir`; if so, leave).

- [ ] **Step 4: Build and run all tests**

Run: `go build ./... && go test ./cli/...`
Expected: build succeeds; tests pass. Any test that referenced `ActiveProfile`, `newProfileCmd`, or `profileLabel` must be deleted as part of this commit.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(cli): remove profile concept — each cwd is its own instance"
```

---

## Task 5: Migrate remaining `~/.hermind` lookups to `InstanceRoot`

**Files:**
- Modify: `cli/bootstrap.go`
- Modify: `cli/models.go`
- Modify: `cli/auth.go`
- Modify: `cli/setup.go`
- Modify: `cli/doctor.go`
- Modify: `cli/skills.go` (audit)
- Modify: `agent/trajectory.go`
- Modify: `agent/prompt.go` (doc string)

- [ ] **Step 1: Update `cli/bootstrap.go:ensureStorage`**

Replace the default-path block:

```go
// Old:
path := app.Config.Storage.SQLitePath
if path == "" {
    home, _ := os.UserHomeDir()
    path = filepath.Join(home, ".hermind", "state.db")
}

// New:
path := app.Config.Storage.SQLitePath
if path == "" {
    if p, err := config.InstancePath("state.db"); err == nil {
        path = p
    } else {
        return fmt.Errorf("hermind: resolve instance root: %w", err)
    }
}
```

Also update the error message on line ~80:
```go
return nil, primaryName, fmt.Errorf("%w: provider %q. Set api_key in <instance>/config.yaml or ANTHROPIC_API_KEY env var", errMissingAPIKey, primaryName)
```

- [ ] **Step 2: Update `cli/models.go:newModelsSwitchCmd`**

Replace:
```go
home, _ := os.UserHomeDir()
cfgPath := filepath.Join(home, ".hermind", "config.yaml")
```
with:
```go
cfgPath, err := config.InstancePath(config.DefaultConfigFile)
if err != nil {
    return err
}
```
Add `"github.com/odysseythink/hermind/config"` to imports. Remove now-unused `"os"`/`"path/filepath"` if they are no longer referenced (`"os"` likely still needed for `os.ReadFile`/`os.WriteFile`; check before deleting).

- [ ] **Step 3: Update `cli/auth.go:defaultCredentialsPath`**

Replace:
```go
func defaultCredentialsPath() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".hermind", "credentials.yaml")
}
```
with:
```go
func defaultCredentialsPath() string {
    p, err := config.InstancePath("credentials.yaml")
    if err != nil {
        return ".hermind/credentials.yaml"
    }
    return p
}
```
Add config import; prune unused imports.

- [ ] **Step 4: Update `cli/setup.go:runSetupInteractive`**

Replace the block starting at the `home, err := os.UserHomeDir()` line inside `runSetupInteractive`:

```go
root, err := config.InstanceRoot()
if err != nil {
    return err
}
if err := os.MkdirAll(root, 0o755); err != nil {
    return err
}
cfgPath := filepath.Join(root, config.DefaultConfigFile)
```

Also update the hint line:
```go
fmt.Println("This writes <instance>/config.yaml. Press Enter to accept defaults.")
```

And fix the default for storagePath prompt:
```go
storagePath := prompt(reader, "SQLite path [<instance>/state.db]", "")
```

When storagePath is empty after the prompt, leave it empty in the template — `resolveDefaults` will fill it on load.

- [ ] **Step 5: Update `cli/doctor.go:checkStorage`**

Replace:
```go
path := app.Config.Storage.SQLitePath
if path == "" {
    home, _ := os.UserHomeDir()
    path = filepath.Join(home, ".hermind", "state.db")
}
```
with:
```go
path := app.Config.Storage.SQLitePath
if path == "" {
    if p, err := config.InstancePath("state.db"); err == nil {
        path = p
    } else {
        return fmt.Errorf("resolve instance root: %w", err)
    }
}
```

- [ ] **Step 6: Update `agent/trajectory.go:DefaultTrajectoryDir`**

Replace the function body:
```go
func DefaultTrajectoryDir() string {
    if v := os.Getenv("HERMIND_HOME"); v != "" {
        return filepath.Join(v, "trajectories")
    }
    cwd, err := os.Getwd()
    if err != nil {
        return ".hermind/trajectories"
    }
    return filepath.Join(cwd, ".hermind", "trajectories")
}
```

(We duplicate the InstanceRoot logic inline here rather than importing config to avoid an agent→config dep cycle. agent/trajectory already has no config import.)

- [ ] **Step 7: Update `agent/prompt.go` doc string**

Find the comment block mentioning `$HERMIND_HOME/skills (defaults to ~/.hermind/skills)` and replace with:
```
<instance-root>/skills (defaults to ./.hermind/skills; override with $HERMIND_HOME)
```

- [ ] **Step 8: Audit `cli/skills.go`**

Grep `cli/skills.go` for `os.UserHomeDir`, `~/.hermind`, `HERMIND_HOME`. For any home-rooted path lookup, replace with `config.InstancePath(...)`. If no hits, leave the file alone.

Run: `grep -n "UserHomeDir\|\.hermind" cli/skills.go`

- [ ] **Step 9: Build and run tests**

Run: `go build ./... && go test ./...`
Expected: build succeeds; tests pass.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "refactor: route all ~/.hermind path lookups through InstanceRoot"
```

---

## Task 6: `cli/listen.go:listenRandomLocalhost` helper

**Files:**
- Create: `cli/listen.go`
- Create: `cli/listen_test.go`

- [ ] **Step 1: Write the failing test**

Create `cli/listen_test.go`:

```go
package cli

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenRandomLocalhost_PicksSomethingInRange(t *testing.T) {
	ln, err := listenRandomLocalhost()
	require.NoError(t, err)
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	assert.Equal(t, "127.0.0.1", addr.IP.String())
	assert.GreaterOrEqual(t, addr.Port, 30000)
	assert.Less(t, addr.Port, 40000)
}

func TestListenRandomLocalhost_RetriesWhenPortBusy(t *testing.T) {
	// Occupy one specific port inside the range and verify the helper
	// still succeeds (by finding any other free port).
	occupier, err := net.Listen("tcp", "127.0.0.1:35000")
	require.NoError(t, err)
	defer occupier.Close()

	ln, err := listenRandomLocalhost()
	require.NoError(t, err)
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	assert.NotEqual(t, 35000, addr.Port)
}

func TestListenRandomLocalhost_ErrorMessageShape(t *testing.T) {
	// Sanity: if we saturate a narrow synthetic range we should see
	// the error string shape. We can't easily saturate [30000,40000)
	// in a test, so we just verify the error wrapping on invalid bind.
	_, err := listenOnRange(1, 0, 1)
	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "no free localhost port") ||
			strings.Contains(err.Error(), "invalid port range"),
		"got %v", err)
}

// helper to prove the range arg is respected — also used to test
// error paths.
var _ = fmt.Sprintf
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli/... -run TestListenRandomLocalhost -v`
Expected: FAIL — `undefined: listenRandomLocalhost`.

- [ ] **Step 3: Write the implementation**

Create `cli/listen.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"syscall"
)

const (
	portMin      = 30000
	portMax      = 40000
	portAttempts = 50
)

// listenRandomLocalhost picks a random TCP port in [portMin, portMax)
// on 127.0.0.1 and returns the bound listener. Retries up to
// portAttempts times on EADDRINUSE before giving up. Any other bind
// error fails immediately.
func listenRandomLocalhost() (net.Listener, error) {
	return listenOnRange(portMin, portMax, portAttempts)
}

// listenOnRange is the underlying helper, split out for testability.
// Callers should use listenRandomLocalhost() in production.
func listenOnRange(minPort, maxPort, attempts int) (net.Listener, error) {
	if minPort <= 0 || maxPort <= minPort {
		return nil, fmt.Errorf("listen: invalid port range [%d,%d)", minPort, maxPort)
	}
	span := maxPort - minPort
	for i := 0; i < attempts; i++ {
		port := minPort + rand.IntN(span)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return ln, nil
		}
		if !isAddrInUse(err) {
			return nil, fmt.Errorf("listen: %w", err)
		}
	}
	return nil, fmt.Errorf("listen: no free localhost port in [%d,%d) after %d attempts",
		minPort, maxPort, attempts)
}

// isAddrInUse reports whether err is the platform's "address already
// in use" sentinel. syscall.EADDRINUSE covers the Linux/macOS cases we
// ship on.
func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./cli/... -run TestListenRandomLocalhost -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/listen.go cli/listen_test.go
git commit -m "feat(cli): listenRandomLocalhost helper (range [30000,40000))"
```

---

## Task 7: Remove token auth from the API layer

**Files:**
- Delete: `api/auth.go`
- Delete: `api/auth_test.go`
- Modify: `api/server.go`
- Modify: `api/server_test.go`

- [ ] **Step 1: Delete auth files**

```bash
git rm api/auth.go api/auth_test.go
```

- [ ] **Step 2: Update `api/server.go`**

Make these edits:

1. Remove the `Token` field from `ServerOpts`:

```go
// Delete these lines from ServerOpts:
//   Token string
```

2. Remove the Token validation in `NewServer`:

```go
// Delete this block:
//   if opts.Token == "" {
//       return nil, fmt.Errorf("api: ServerOpts.Token is required")
//   }
```

3. In `buildRouter()`, remove the auth middleware wiring. The new `buildRouter` method becomes:

```go
func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Get("/model/info", s.handleModelInfo)

		r.Get("/config", s.handleConfigGet)
		r.Put("/config", s.handleConfigPut)

		r.Get("/sessions", s.handleSessionsList)
		r.Get("/sessions/{id}", s.handleSessionGet)
		r.Patch("/sessions/{id}", s.handleSessionPatch)
		r.Delete("/sessions/{id}", s.handleSessionDelete)
		r.Get("/sessions/{id}/messages", s.handleSessionMessages)
		r.Post("/sessions/{id}/messages", s.handleSessionMessagesPost)
		r.Post("/sessions/{id}/cancel", s.handleSessionCancel)
		r.Get("/sessions/{id}/stream/ws", s.handleSessionStreamWS)
		r.Get("/sessions/{id}/stream/sse", s.handleSessionStreamSSE)

		r.Get("/tools", s.handleToolsList)
		r.Get("/skills", s.handleSkillsList)
		r.Get("/providers", s.handleProvidersList)
		r.Post("/providers/{name}/models", s.handleProvidersModels)
		r.Post("/fallback_providers/{index}/models", s.handleFallbackProvidersModels)
		r.Get("/config/schema", s.handleConfigSchema)
		r.Get("/platforms/schema", s.handlePlatformsSchema)
		r.Post("/platforms/{key}/reveal", s.handlePlatformReveal)
		r.Post("/platforms/{key}/test", s.handlePlatformTest)
		r.Post("/platforms/apply", s.handlePlatformsApply)
	})

	r.Get("/", s.handleIndex)
	r.Get("/ui/*", s.handleStatic)

	return r
}
```

4. Update `handleIndex` to stop injecting `{{TOKEN}}`:

```go
func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	data, err := fs.ReadFile(webroot, "webroot/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
```

5. Remove now-unused imports (`strings`, any others only used for token substitution).

- [ ] **Step 3: Update `api/server_test.go`**

Open `api/server_test.go` and remove every `Token: "..."` entry from `ServerOpts` literals. If any tests set the `Authorization` header or verify 401 responses for missing tokens, delete them entirely (they no longer apply).

Run this one-liner to spot residual references:
```
grep -n "Token:\|Bearer \|?t=\|Authorization" api/*.go
```

Follow-up and clean any lingering references in `api/` to token/auth.

- [ ] **Step 4: Build and run tests**

Run: `go build ./... && go test ./api/...`
Expected: build succeeds; tests pass.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(api): remove Bearer token auth — localhost-only bind is sole gate"
```

---

## Task 8: `cli/web.go` uses random port + drops token

**Files:**
- Modify: `cli/web.go`
- Modify: `cli/root.go`
- Modify: `cli/web_test.go` (if present)

- [ ] **Step 1: Rewrite `runWeb` in `cli/web.go`**

Replace the `runWeb` function with the following body. Key changes: no `api.GenerateToken`, no `Token` field on `ServerOpts`, use `listenRandomLocalhost()` when `opts.Addr` is empty, updated banner.

```go
func runWeb(ctx context.Context, app *App, opts webRunOptions) error {
	if err := ensureStorage(app); err != nil {
		return err
	}

	deps, cleanup, err := BuildEngineDeps(ctx, app)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil && !errors.Is(err, errMissingAPIKey) {
		return fmt.Errorf("web: build engine deps: %w", err)
	}

	ctrl := gatewayctl.New(app.Config, func(cfg config.Config) (*gateway.Gateway, error) {
		return BuildGateway(BuildGatewayDeps{
			Config:  cfg,
			Primary: deps.Provider,
			Aux:     deps.AuxProvider,
			Storage: deps.Storage,
			Tools:   deps.ToolReg,
		})
	})
	if err := ctrl.Start(ctx); err != nil {
		return fmt.Errorf("web: start gateway controller: %w", err)
	}
	defer func() {
		shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
		defer c2()
		ctrl.Shutdown(shutCtx)
	}()

	streams := api.NewMemoryStreamHub()
	srv, err := api.NewServer(&api.ServerOpts{
		Config:     app.Config,
		ConfigPath: app.ConfigPath,
		Storage:    app.Storage,
		Version:    Version,
		Streams:    streams,
		Controller: ctrl,
		Deps:       deps,
	})
	if err != nil {
		return err
	}

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
	realAddr := "http://" + ln.Addr().String()
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintf(out, "hermind web listening on %s\n", realAddr)
	fmt.Fprintf(out, "instance:  %s\n", app.InstanceRoot)
	fmt.Fprintf(out, "open:      %s/\n", realAddr)

	if !opts.NoBrowser {
		go openBrowser(realAddr + "/")
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
```

- [ ] **Step 2: Update `newWebCmd` default flag value**

In `newWebCmd`, change the `--addr` flag default from `"127.0.0.1:9119"` to `""`:

```go
c.Flags().StringVar(&opts.Addr, "addr", "",
    "bind address; empty = random port in [30000,40000) on 127.0.0.1")
```

Update the `Long` description:
```go
Long: `Start the hermind web UI and REST API.

Binds to 127.0.0.1 by default on a random port in [30000,40000). Use
--addr to pin a specific host:port (useful for bookmarks or reverse
proxies).`,
```

Update the top comment referring to "listening-URL and token banner lines" — remove the word "token".

- [ ] **Step 3: Update `cli/root.go`**

Find the `root.RunE` default. Change:
```go
return runWeb(cmd.Context(), app, webRunOptions{
    Addr: "127.0.0.1:9119",
    Out:  cmd.OutOrStdout(),
})
```
to:
```go
return runWeb(cmd.Context(), app, webRunOptions{
    Out: cmd.OutOrStdout(),
})
```

- [ ] **Step 4: Update `cli/web_test.go`** (if it exists)

Grep: `grep -l "9119\|GenerateToken\|Token:\|?t=" cli/web_test.go`. Remove references; tests that set `Addr: "127.0.0.1:9119"` should leave `Addr` empty or pick an explicit test port like `"127.0.0.1:0"`.

- [ ] **Step 5: Build and smoke-test**

Run: `go build ./... && go test ./cli/...`
Expected: tests pass.

Optional smoke test — manually:
```bash
( cd /tmp/ws-a && go run ./cmd/hermind web --exit-after 2s --no-browser )
( cd /tmp/ws-b && go run ./cmd/hermind web --exit-after 2s --no-browser )
```
Expected: two banners each printing a different `http://127.0.0.1:3XXXX` port and different instance paths.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(cli): random localhost port + drop token from web banner"
```

---

## Task 9: `/api/status` exposes `instance_root`

**Files:**
- Modify: `api/dto.go`
- Modify: `api/handlers_meta.go`
- Modify: `api/server.go`

- [ ] **Step 1: Extend `StatusResponse` DTO**

In `api/dto.go`, update:

```go
type StatusResponse struct {
	Version       string `json:"version"`
	UptimeSec     int64  `json:"uptime_sec"`
	StorageDriver string `json:"storage_driver"`
	InstanceRoot  string `json:"instance_root"`
}
```

- [ ] **Step 2: Extend `ServerOpts` with `InstanceRoot`**

In `api/server.go`, add `InstanceRoot` to `ServerOpts`:

```go
// InstanceRoot is the absolute path to this hermind instance's root
// directory. Surfaced via GET /api/status so the UI can display it.
InstanceRoot string
```

- [ ] **Step 3: Update `handleStatus`**

In `api/handlers_meta.go`:

```go
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, StatusResponse{
		Version:       s.opts.Version,
		UptimeSec:     int64(time.Since(s.bootedAt).Seconds()),
		StorageDriver: s.driverName(),
		InstanceRoot:  s.opts.InstanceRoot,
	})
}
```

- [ ] **Step 4: Wire `cli/web.go` to pass `InstanceRoot`**

In `cli/web.go:runWeb`, add `InstanceRoot: app.InstanceRoot` to the `api.NewServer(&api.ServerOpts{...})` literal.

- [ ] **Step 5: Write a test**

Append to `api/server_test.go`:

```go
func TestHandleStatus_IncludesInstanceRoot(t *testing.T) {
	srv, err := NewServer(&ServerOpts{
		Config:       &config.Config{},
		Version:      "test",
		InstanceRoot: "/tmp/test/.hermind",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body StatusResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "/tmp/test/.hermind", body.InstanceRoot)
}
```

(Add imports `"encoding/json"`, `"net/http"`, `"net/http/httptest"` as needed.)

- [ ] **Step 6: Run tests**

Run: `go test ./api/... -run TestHandleStatus_IncludesInstanceRoot -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(api): GET /api/status returns instance_root"
```

---

## Task 10: Frontend — drop token, strip `{{TOKEN}}`, drop `?t=`

**Files:**
- Modify: `web/index.html`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/hooks/useChatStream.ts`

- [ ] **Step 1: Strip `{{TOKEN}}` from `web/index.html`**

Remove the entire `<script>` block containing `window.HERMIND = { token: "{{TOKEN}}" };`. The `<head>` should no longer have that inline script.

- [ ] **Step 2: Rewrite `web/src/api/client.ts`**

Replace the file:

```ts
import type { z } from 'zod';

/** Thrown for any non-2xx response; carries the decoded JSON error if present. */
export class ApiError extends Error {
  constructor(public status: number, public body: unknown) {
    super(`api: ${status}`);
  }
}

/**
 * apiFetch sends a JSON request to hermind. No auth header is attached;
 * hermind binds to 127.0.0.1 only, and access is gated by localhost
 * reachability.
 */
export async function apiFetch<T>(
  path: string,
  opts: {
    method?: string;
    body?: unknown;
    schema?: z.ZodType<T>;
    signal?: AbortSignal;
  } = {},
): Promise<T> {
  const res = await fetch(path, {
    method: opts.method ?? 'GET',
    headers: { 'Content-Type': 'application/json' },
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
  });

  const ctype = res.headers.get('content-type') ?? '';
  const parsed = ctype.includes('application/json') ? await res.json() : await res.text();

  if (!res.ok) {
    throw new ApiError(res.status, parsed);
  }

  if (opts.schema) {
    return opts.schema.parse(parsed);
  }
  return parsed as T;
}
```

- [ ] **Step 3: Update `web/src/hooks/useChatStream.ts`**

Open the file. Line ~29 looks like:
```ts
`/api/sessions/${encodeURIComponent(sessionId)}/stream/sse?t=${encodeURIComponent(token)}`
```

Remove `?t=${encodeURIComponent(token)}`. Also remove any `resolveToken()` import and any `const token = resolveToken()` line. The `sessionId` usage remains (it gets removed in PR 2).

Run `grep -n "?t=\|resolveToken\|token" web/src/hooks/useChatStream.ts` to confirm no orphaned references remain.

- [ ] **Step 4: Search for any remaining frontend token references**

```bash
grep -rn "resolveToken\|VITE_HERMIND_TOKEN\|{{TOKEN}}\|?t=" web/src web/index.html
```
Expected: no hits after this task.

- [ ] **Step 5: Run frontend tests**

```bash
cd web && pnpm test --run
```
Expected: all tests pass. Any test asserting token behavior should be deleted.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(web): drop token from client, hooks, and index.html"
```

---

## Task 11: Frontend — `ConversationHeader` shows instance path

**Files:**
- Modify: `web/src/components/chat/ConversationHeader.tsx`
- Modify: `web/src/components/chat/ConversationHeader.module.css`

- [ ] **Step 1: Update `ConversationHeader.tsx`**

Replace the contents:

```tsx
import { useTranslation } from 'react-i18next';
import SettingsButton from './SettingsButton';
import styles from './ConversationHeader.module.css';

type Props = {
  title: string;
  instanceRoot: string;
  onOpenSettings: () => void;
  settingsDisabled?: boolean;
};

export default function ConversationHeader({
  title,
  instanceRoot,
  onOpenSettings,
  settingsDisabled,
}: Props) {
  const { t } = useTranslation('ui');
  return (
    <header className={styles.header}>
      <span
        className={styles.instancePath}
        title={instanceRoot}
        dir="rtl"
        aria-label={t('chat.instance.label', { defaultValue: 'Instance' })}
      >
        {instanceRoot}
      </span>
      <h2 className={styles.title}>{title}</h2>
      <SettingsButton
        onClick={onOpenSettings}
        disabled={settingsDisabled}
        ariaLabel={t('chat.settings.title')}
      />
    </header>
  );
}
```

- [ ] **Step 2: Update `ConversationHeader.module.css`**

Add these rules (keep existing `.header` rule, add the new `.instancePath`):

```css
.header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border-subtle);
}

.instancePath {
  font-family: var(--font-mono, 'JetBrains Mono', monospace);
  font-size: 13px;
  color: var(--fg-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 40ch;
  direction: rtl;
  unicode-bidi: plaintext;
}

.title {
  flex: 1;
  text-align: center;
  font-size: 14px;
  font-family: var(--font-mono, 'JetBrains Mono', monospace);
  margin: 0;
}
```

(Adjust the existing `.header` / `.title` rules if the file already defines them — keep the semantics above.)

- [ ] **Step 3: Thread `instanceRoot` through the component tree**

Find the callers of `ConversationHeader` (likely `ChatWorkspace.tsx`). Each must receive `instanceRoot` as a prop and pass it through.

Edit `ChatWorkspace.tsx` to accept an `instanceRoot: string` prop and forward it:
```tsx
<ConversationHeader
  title={title}
  instanceRoot={instanceRoot}
  onOpenSettings={onOpenSettings}
  settingsDisabled={settingsDisabled}
/>
```

- [ ] **Step 4: Update `App.tsx` to fetch and pass `instanceRoot`**

In `App.tsx`, add a small fetch for `/api/status` on mount:

```tsx
import { useEffect, useState } from 'react';
import { apiFetch } from './api/client';

// inside App():
const [instanceRoot, setInstanceRoot] = useState<string>('');

useEffect(() => {
  apiFetch<{ instance_root: string }>('/api/status')
    .then((s) => setInstanceRoot(s.instance_root ?? ''))
    .catch(() => {/* leave empty; header shows nothing */});
}, []);
```

Pass `instanceRoot={instanceRoot}` down to `ChatWorkspace`.

- [ ] **Step 5: Update any `ConversationHeader.test.tsx` / `ChatWorkspace.test.tsx`**

If tests exist, add an `instanceRoot` prop to all mounts, e.g. `instanceRoot="/tmp/test/.hermind"`. Add an assertion to `ConversationHeader.test.tsx`:

```tsx
it('renders the instance path with rtl direction', () => {
  render(
    <ConversationHeader
      title="hello"
      instanceRoot="/Users/me/proj/.hermind"
      onOpenSettings={() => {}}
    />
  );
  const el = screen.getByText('/Users/me/proj/.hermind');
  expect(el).toHaveAttribute('dir', 'rtl');
  expect(el).toHaveAttribute('title', '/Users/me/proj/.hermind');
});
```

- [ ] **Step 6: Run frontend tests**

```bash
cd web && pnpm test --run
```
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(web): ConversationHeader displays instance absolute path"
```

---

## Task 12: Frontend — set `document.title` to include instance segment

**Files:**
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Update `main.tsx` to fetch and set title**

```tsx
import React from 'react';
import { createRoot } from 'react-dom/client';
import './styles/theme.css';
import 'katex/dist/katex.min.css';
import App from './App';
import { initI18n } from './i18n';

const rootElem = document.getElementById('root');
if (!rootElem) {
  throw new Error('hermind: #root element missing');
}

function setTitleFromInstance(instanceRoot: string) {
  if (!instanceRoot) return;
  const parts = instanceRoot.split('/').filter(Boolean);
  const parent = parts.length >= 2 ? `${parts[parts.length - 2]}/${parts[parts.length - 1]}`
    : instanceRoot;
  document.title = `hermind — ${parent}`;
}

// Fetch /api/status early so the tab title reflects the instance even
// before React mounts. Failures are silent — tab keeps the default.
fetch('/api/status')
  .then((r) => r.json())
  .then((s: { instance_root?: string }) => setTitleFromInstance(s.instance_root ?? ''))
  .catch(() => {});

initI18n()
  .catch((err) => console.error('i18n init failed:', err))
  .finally(() => {
    createRoot(rootElem).render(
      <React.StrictMode>
        <App />
      </React.StrictMode>,
    );
  });
```

- [ ] **Step 2: Verify**

Run `cd web && pnpm test --run`. Expected: PASS. Then smoke-test via `pnpm dev` and observe the browser tab title updates once `/api/status` responds.

- [ ] **Step 3: Commit**

```bash
git add web/src/main.tsx
git commit -m "feat(web): document.title reflects instance path end-segment"
```

---

## Task 13: CHANGELOG + webroot rebuild + final verification

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `api/webroot/*` (via `pnpm build` output copy)

- [ ] **Step 1: Add CHANGELOG entry**

Open `CHANGELOG.md` and prepend under the unreleased section:

```markdown
## Unreleased

### Breaking
- Config directory is now `./.hermind/` (cwd-rooted). The legacy `~/.hermind/`
  path is no longer read. `HERMIND_HOME` still overrides. A one-time stderr
  hint is printed on first boot if `~/.hermind/` exists.
- Web server binds to a random port in `[30000, 40000)` instead of the fixed
  `9119`. Use `--addr host:port` to pin.
- Bearer token auth removed from the web UI. Access is gated solely by
  127.0.0.1 binding.
- `HERMIND_PROFILE` env var and `profiles/<name>/` directory layout removed.
  The `hermind profile` subcommand tree is gone — each cwd is its own profile.

### Added
- `GET /api/status` now returns `instance_root`.
- Frontend displays the absolute instance path in the conversation header
  and in the browser tab title.
```

- [ ] **Step 2: Rebuild the frontend bundle**

```bash
cd web && pnpm install && pnpm build
```

- [ ] **Step 3: Copy bundle into `api/webroot/`**

This repo copies the dist output into `api/webroot/` so the Go embed picks it up. Follow the existing convention — likely a script or manual copy. Check `api/webroot/` for an `index.html`; replace it and the `assets/` tree with the new `web/dist/` contents.

```bash
rm -rf api/webroot/*
cp -r web/dist/. api/webroot/
```

- [ ] **Step 4: Full test run**

```bash
go test ./...
cd web && pnpm test --run && cd ..
```
Expected: all green.

- [ ] **Step 5: Smoke test — two independent instances**

```bash
mkdir -p /tmp/ws-a /tmp/ws-b
( cd /tmp/ws-a && go run ./cmd/hermind web --exit-after 3s --no-browser ) &
( cd /tmp/ws-b && go run ./cmd/hermind web --exit-after 3s --no-browser ) &
wait
ls /tmp/ws-a/.hermind /tmp/ws-b/.hermind
```
Expected: both directories exist with their own `config.yaml` and (after the instance runs) `state.db`. The two banners print distinct ports in `[30000,40000)`.

- [ ] **Step 6: Commit**

```bash
git add CHANGELOG.md api/webroot/
git commit -m "chore: CHANGELOG + rebuild webroot bundle for PR 1"
```

- [ ] **Step 7: Open PR**

```bash
gh pr create --title "refactor: cwd-rooted instance config + random port + drop token auth" --body "$(cat <<'EOF'
## Summary
- Introduces `config.InstanceRoot()` — config resolution is now `$HERMIND_HOME` → `./.hermind`, no home-dir fallback.
- Removes the `profile` concept; each cwd is its own profile.
- Random port in `[30000, 40000)`; `--addr` escape hatch preserved.
- Drops Bearer token auth; server is localhost-only, which is the sole access gate.
- Frontend: instance absolute path in header and tab title.
- `/api/status` surfaces `instance_root` for the UI.

## Test plan
- [ ] `go test ./...` passes
- [ ] `cd web && pnpm test --run` passes
- [ ] Two instances started from different cwds bind to distinct random ports and have independent `.hermind/` directories
- [ ] With a pre-existing `~/.hermind/`, a fresh instance in a new cwd prints the migration hint once and creates `<cwd>/.hermind/.migration_notice_shown`
- [ ] `HERMIND_HOME=/foo hermind web` uses `/foo` as the instance root
- [ ] Opening the UI requires no token; no `Authorization` header or `?t=` in requests

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## PR 1 Acceptance Checklist

Verify each of the spec's PR 1 acceptance criteria:

- [ ] Two concurrent instances from different cwds bind to distinct random ports with independent `.hermind/`.
- [ ] First-run stderr hint fires exactly once per instance root when `~/.hermind/` exists.
- [ ] `HERMIND_HOME=/some/path hermind web` uses the given path verbatim.
- [ ] Header shows absolute instance path; `document.title` shows the trailing path segment.
- [ ] No `?t=` or `Authorization` in requests; server binds only `127.0.0.1`.
- [ ] `HERMIND_PROFILE` and `profiles/<name>/` code paths are gone (`git grep HERMIND_PROFILE` returns nothing).
- [ ] `GET /api/status` includes `instance_root`.
- [ ] `go test ./...` and `pnpm test --run` pass.
