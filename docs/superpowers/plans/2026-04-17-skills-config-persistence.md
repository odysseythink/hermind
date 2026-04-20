# Skills Config Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist skill enable/disable state into `~/.hermind/config.yaml` (or the active profile's `config.yaml`) so the user's selections survive across runs, matching the Python behavior (`skills.disabled` + `skills.platform_disabled`).

**Architecture:** Add a new `SkillsConfig` block to `config.Config`. Extend `skills.Registry` with helpers that consult the config to decide initial active state. Replace the stub `skillsPersistActive` in `cli/skills.go` with a real config round-trip writer. Wire REPL and gateway boot-up paths to call `LoadActiveFromConfig` so persisted enablement is honored at startup. Reuse the existing YAML loader (`config.LoadFromPath`); save via a new `config.SaveToPath` helper.

**Tech Stack:** Go 1.21+, `gopkg.in/yaml.v3`, existing `skills/` and `config/` packages.

---

## File Structure

- Modify: `config/config.go` — add `SkillsConfig` type + field on `Config`
- Create: `config/save.go` — new `SaveToPath(path string, cfg *Config) error`
- Create: `config/save_test.go`
- Modify: `skills/registry.go` — add `ApplyConfig`, `Disabled`, `SetDisabled`, `IsDisabled`, `ListDisabled` helpers
- Modify: `skills/registry.go` test file (`skills/skill_test.go`) — add tests for new helpers
- Create: `skills/config.go` — pure helpers `DisabledForPlatform(cfg, platform)` and `WithDisabledUpdate(cfg, name, platform, disabled)` that operate on `config.SkillsConfig`
- Create: `skills/config_test.go`
- Modify: `cli/skills.go` — replace stub `skillsPersistActive` with real writer; update `loadSkills` to activate non-disabled skills based on config
- Modify: `cli/app.go` — expose the source config path as `App.ConfigPath` so `cli/skills.go` can save back
- Modify: `cmd/hermind/main.go` — nothing needed (indirect)

---

## Task 1: Add `SkillsConfig` block to Config

**Files:**
- Modify: `config/config.go`
- Test: `config/loader_test.go` (existing)

- [ ] **Step 1: Write the failing test**

Append to `config/loader_test.go`:

```go
func TestLoadFromPath_Skills(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := []byte(`
model: anthropic/claude-opus-4-6
providers: {}
agent:
  max_turns: 10
terminal:
  backend: local
storage:
  driver: sqlite
skills:
  disabled:
    - foo
    - bar
  platform_disabled:
    cli: [baz]
    gateway: [qux]
`)
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Skills.Disabled) != 2 || cfg.Skills.Disabled[0] != "foo" {
		t.Errorf("disabled = %v", cfg.Skills.Disabled)
	}
	if got := cfg.Skills.PlatformDisabled["cli"]; len(got) != 1 || got[0] != "baz" {
		t.Errorf("platform_disabled[cli] = %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestLoadFromPath_Skills -v`
Expected: FAIL with `cfg.Skills undefined`

- [ ] **Step 3: Add the config type**

In `config/config.go`, inside the `Config` struct (after `Tracing`):

```go
	Skills            SkillsConfig              `yaml:"skills,omitempty"`
```

Then at the bottom of the file:

```go
// SkillsConfig records user skill enable/disable selections. It mirrors
// the Python hermes config layout so the same config.yaml works for both.
// An empty struct means "every discovered skill is active".
type SkillsConfig struct {
	// Disabled is the list of skill names disabled on every platform.
	Disabled []string `yaml:"disabled,omitempty"`
	// PlatformDisabled is a per-platform override layered on top of Disabled.
	// Keys match the string passed to the CLI/REPL/gateway startup path
	// (e.g. "cli", "gateway", "cron").
	PlatformDisabled map[string][]string `yaml:"platform_disabled,omitempty"`
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./config/ -run TestLoadFromPath_Skills -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/loader_test.go
git commit -m "feat(config): add skills disable/platform_disabled block"
```

---

## Task 2: Write config back to disk

**Files:**
- Create: `config/save.go`
- Create: `config/save_test.go`

- [ ] **Step 1: Write the failing test**

Create `config/save_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveToPath_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Skills = SkillsConfig{
		Disabled: []string{"alpha"},
		PlatformDisabled: map[string][]string{
			"cli": {"beta"},
		},
	}
	if err := SaveToPath(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	back, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(back.Skills.Disabled) != 1 || back.Skills.Disabled[0] != "alpha" {
		t.Errorf("disabled = %v", back.Skills.Disabled)
	}
	if got := back.Skills.PlatformDisabled["cli"]; len(got) != 1 || got[0] != "beta" {
		t.Errorf("platform_disabled = %v", back.Skills.PlatformDisabled)
	}
}

func TestSaveToPath_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "sub", "config.yaml")
	if err := SaveToPath(path, Default()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestSaveToPath -v`
Expected: FAIL with `SaveToPath undefined`

- [ ] **Step 3: Implement SaveToPath**

Create `config/save.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveToPath writes cfg as YAML to path. It creates parent directories
// as needed and writes atomically via a temp file + rename so a crash
// mid-write never leaves a half-written config.
func SaveToPath(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config: SaveToPath: nil config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", filepath.Dir(path), err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("config: create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Ensure temp is cleaned up on failure.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("config: rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestSaveToPath -v`
Expected: PASS (both sub-tests)

- [ ] **Step 5: Commit**

```bash
git add config/save.go config/save_test.go
git commit -m "feat(config): add atomic SaveToPath writer"
```

---

## Task 3: Registry helpers that consume SkillsConfig

**Files:**
- Create: `skills/config.go`
- Create: `skills/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `skills/config_test.go`:

```go
package skills

import (
	"reflect"
	"sort"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestDisabledForPlatform_Union(t *testing.T) {
	cfg := config.SkillsConfig{
		Disabled: []string{"always-off"},
		PlatformDisabled: map[string][]string{
			"cli":     {"cli-only"},
			"gateway": {"gateway-only"},
		},
	}
	got := DisabledForPlatform(cfg, "cli")
	sort.Strings(got)
	want := []string{"always-off", "cli-only"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDisabledForPlatform_EmptyPlatform(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"a"}}
	got := DisabledForPlatform(cfg, "")
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("got %v", got)
	}
}

func TestWithDisabledUpdate_AddGlobal(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"x"}}
	got := WithDisabledUpdate(cfg, "y", "", true)
	want := []string{"x", "y"}
	if !reflect.DeepEqual(got.Disabled, want) {
		t.Errorf("got %v, want %v", got.Disabled, want)
	}
}

func TestWithDisabledUpdate_RemoveGlobal(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"x", "y"}}
	got := WithDisabledUpdate(cfg, "y", "", false)
	if len(got.Disabled) != 1 || got.Disabled[0] != "x" {
		t.Errorf("got %v", got.Disabled)
	}
}

func TestWithDisabledUpdate_AddPlatform(t *testing.T) {
	cfg := config.SkillsConfig{}
	got := WithDisabledUpdate(cfg, "y", "cli", true)
	if len(got.PlatformDisabled["cli"]) != 1 || got.PlatformDisabled["cli"][0] != "y" {
		t.Errorf("got %v", got.PlatformDisabled)
	}
}

func TestWithDisabledUpdate_RemovePlatform(t *testing.T) {
	cfg := config.SkillsConfig{
		PlatformDisabled: map[string][]string{"cli": {"y", "z"}},
	}
	got := WithDisabledUpdate(cfg, "y", "cli", false)
	if len(got.PlatformDisabled["cli"]) != 1 || got.PlatformDisabled["cli"][0] != "z" {
		t.Errorf("got %v", got.PlatformDisabled)
	}
}

func TestWithDisabledUpdate_NoDuplicate(t *testing.T) {
	cfg := config.SkillsConfig{Disabled: []string{"x"}}
	got := WithDisabledUpdate(cfg, "x", "", true)
	if len(got.Disabled) != 1 {
		t.Errorf("expected no duplicate, got %v", got.Disabled)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./skills/ -run TestDisabledForPlatform -v`
Expected: FAIL with `DisabledForPlatform undefined`

- [ ] **Step 3: Implement the helpers**

Create `skills/config.go`:

```go
package skills

import (
	"sort"

	"github.com/odysseythink/hermind/config"
)

// DisabledForPlatform returns the union of global Disabled and the
// platform-specific override list, deduplicated. Order is not
// guaranteed — callers that care should sort.
func DisabledForPlatform(cfg config.SkillsConfig, platform string) []string {
	seen := make(map[string]struct{}, len(cfg.Disabled))
	for _, n := range cfg.Disabled {
		seen[n] = struct{}{}
	}
	if platform != "" {
		for _, n := range cfg.PlatformDisabled[platform] {
			seen[n] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	return out
}

// WithDisabledUpdate returns a new SkillsConfig value with the given
// skill's disabled state flipped. If platform is "", the global
// Disabled list is edited; otherwise the platform-specific override
// list is edited.
//
// disabled=true → the skill gets added to the list (if not present).
// disabled=false → the skill gets removed (if present).
func WithDisabledUpdate(cfg config.SkillsConfig, name, platform string, disabled bool) config.SkillsConfig {
	out := cfg
	if platform == "" {
		out.Disabled = updateStringList(cfg.Disabled, name, disabled)
		return out
	}
	if out.PlatformDisabled == nil {
		out.PlatformDisabled = map[string][]string{}
	} else {
		// shallow copy of the map so callers can't see partial mutation
		cp := make(map[string][]string, len(out.PlatformDisabled))
		for k, v := range out.PlatformDisabled {
			cp[k] = append([]string(nil), v...)
		}
		out.PlatformDisabled = cp
	}
	out.PlatformDisabled[platform] = updateStringList(out.PlatformDisabled[platform], name, disabled)
	if len(out.PlatformDisabled[platform]) == 0 {
		delete(out.PlatformDisabled, platform)
	}
	return out
}

func updateStringList(list []string, name string, add bool) []string {
	seen := false
	out := make([]string, 0, len(list)+1)
	for _, n := range list {
		if n == name {
			seen = true
			if add {
				out = append(out, n) // keep single copy
			}
			continue
		}
		out = append(out, n)
	}
	if add && !seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./skills/ -run "TestDisabledForPlatform|TestWithDisabledUpdate" -v`
Expected: PASS (all 7 sub-tests)

- [ ] **Step 5: Commit**

```bash
git add skills/config.go skills/config_test.go
git commit -m "feat(skills): add DisabledForPlatform + WithDisabledUpdate helpers"
```

---

## Task 4: Registry ApplyConfig

**Files:**
- Modify: `skills/registry.go`
- Test: `skills/skill_test.go`

- [ ] **Step 1: Write the failing test**

Append to `skills/skill_test.go`:

```go
func TestRegistryApplyConfig_ActivatesUnlessDisabled(t *testing.T) {
	r := NewRegistry()
	r.Add(&Skill{Name: "a"})
	r.Add(&Skill{Name: "b"})
	r.Add(&Skill{Name: "c"})

	r.ApplyConfig([]string{"b"})

	active := r.Active()
	if len(active) != 2 {
		t.Fatalf("expected 2 active, got %v", active)
	}
	got := []string{active[0].Name, active[1].Name}
	want := []string{"a", "c"}
	if got[0] != want[0] || got[1] != want[1] {
		t.Errorf("active = %v, want %v", got, want)
	}
}

func TestRegistryApplyConfig_UnknownNameIgnored(t *testing.T) {
	r := NewRegistry()
	r.Add(&Skill{Name: "a"})
	// "ghost" does not exist — must not panic or block "a"
	r.ApplyConfig([]string{"ghost"})
	if got := r.Active(); len(got) != 1 || got[0].Name != "a" {
		t.Errorf("active = %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./skills/ -run TestRegistryApplyConfig -v`
Expected: FAIL with `r.ApplyConfig undefined`

- [ ] **Step 3: Implement ApplyConfig**

Append to `skills/registry.go`:

```go
// ApplyConfig sets the registry's active set to "all known skills
// minus disabled". It replaces whatever was previously active. Names
// in disabled that don't correspond to a registered skill are silently
// ignored — this mirrors Python's behavior and keeps startup resilient
// against stale config entries.
func (r *Registry) ApplyConfig(disabled []string) {
	drop := make(map[string]struct{}, len(disabled))
	for _, n := range disabled {
		drop[n] = struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = make(map[string]bool, len(r.skills))
	for name := range r.skills {
		if _, off := drop[name]; off {
			continue
		}
		r.active[name] = true
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./skills/ -run TestRegistryApplyConfig -v`
Expected: PASS (both sub-tests)

- [ ] **Step 5: Commit**

```bash
git add skills/registry.go skills/skill_test.go
git commit -m "feat(skills): add Registry.ApplyConfig for config-driven activation"
```

---

## Task 5: Expose ConfigPath on App

**Files:**
- Modify: `cli/app.go`

- [ ] **Step 1: Write the failing test**

Create `cli/app_test.go` (new file — `cli/` has no existing app test):

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewApp_ConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Pre-create a minimal config so NewApp skips first-run.
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("model: anthropic/claude-opus-4-6\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	defer app.Close()

	if app.ConfigPath != cfgPath {
		t.Errorf("ConfigPath = %q, want %q", app.ConfigPath, cfgPath)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestNewApp_ConfigPath -v`
Expected: FAIL with `app.ConfigPath undefined` OR failing because `defaultConfigPath` ignores `HERMIND_HOME`

- [ ] **Step 3: Audit defaultConfigPath**

Run: `grep -n defaultConfigPath cli/*.go` and read the function. If `HERMIND_HOME` is not honored, extend it to use `$HERMIND_HOME/config.yaml` when set, matching `skills.DefaultHome` behavior. If already honored, skip to Step 4.

Expected shape of `defaultConfigPath` after the edit:

```go
func defaultConfigPath() (string, error) {
	if v := os.Getenv("HERMIND_HOME"); v != "" {
		return filepath.Join(v, "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hermind", "config.yaml"), nil
}
```

- [ ] **Step 4: Add ConfigPath to App**

In `cli/app.go`, update `App` struct and `NewApp`:

```go
type App struct {
	Config     *config.Config
	ConfigPath string          // absolute path the config was loaded from
	Storage    storage.Storage
}

func NewApp() (*App, error) {
	path, err := defaultConfigPath()
	if err != nil {
		return nil, err
	}

	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "no config found — launching first-run setup...")
		if err := configui.RunFirstRun(path); err != nil {
			return nil, fmt.Errorf("first-run setup: %w", err)
		}
	}

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		return nil, err
	}
	return &App{Config: cfg, ConfigPath: path}, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./cli/ -run TestNewApp_ConfigPath -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cli/app.go cli/app_test.go
git commit -m "feat(cli): expose App.ConfigPath for config write-back"
```

---

## Task 6: Wire skills CLI to persist via config

**Files:**
- Modify: `cli/skills.go`
- Test: `cli/skills_test.go` (new file)

- [ ] **Step 1: Write the failing test**

Create `cli/skills_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestSkillsEnableDisable_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Seed: skills dir with two skills.
	seed := func(name string) {
		p := filepath.Join(dir, "skills", "cat", name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: test\n---\nbody"
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed("alpha")
	seed("beta")

	// Seed config so NewApp does not launch first-run.
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("model: anthropic/test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// Disable alpha via the CLI helper.
	if err := skillsPersistActive(app, "alpha", false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	// Config file now lists alpha as disabled.
	back, err := config.LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(back.Skills.Disabled) != 1 || back.Skills.Disabled[0] != "alpha" {
		t.Errorf("disabled = %v", back.Skills.Disabled)
	}

	// Re-enable alpha — disabled list is now empty.
	if err := skillsPersistActive(app, "alpha", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	back, _ = config.LoadFromPath(cfgPath)
	if len(back.Skills.Disabled) != 0 {
		t.Errorf("expected empty disabled, got %v", back.Skills.Disabled)
	}

	// Unknown skill returns an error and does not mutate config.
	var stderr bytes.Buffer
	if err := skillsPersistActiveWithOut(app, "ghost", true, &stderr); err == nil {
		t.Errorf("expected error for unknown skill")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestSkillsEnableDisable_RoundTrip -v`
Expected: FAIL — `skillsPersistActive` currently has signature `(string, bool) error` and only prints; new test expects `(*App, string, bool) error`.

- [ ] **Step 3: Rewrite skillsPersistActive**

Replace the stub in `cli/skills.go` with a real writer. Full new file contents:

```go
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/skills"
	"github.com/spf13/cobra"
)

// newSkillsCmd creates the "hermind skills" subcommand tree.
func newSkillsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage hermind skill packages",
	}
	cmd.AddCommand(newSkillsListCmd(app))
	cmd.AddCommand(newSkillsEnableCmd(app))
	cmd.AddCommand(newSkillsDisableCmd(app))
	return cmd
}

func newSkillsListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _ := loadSkills(app)
			all := reg.All()
			if len(all) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no skills installed")
				fmt.Fprintf(cmd.OutOrStdout(), "install skills into %s\n", skills.DefaultHome())
				return nil
			}
			disabled := skills.DisabledForPlatform(app.Config.Skills, "")
			off := make(map[string]struct{}, len(disabled))
			for _, n := range disabled {
				off[n] = struct{}{}
			}
			for _, s := range all {
				marker := "●"
				if _, d := off[s.Name]; d {
					marker = "○"
				}
				desc := s.Description
				if len(desc) > 68 {
					desc = desc[:68] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %-32s %s\n", marker, s.Name, desc)
			}
			return nil
		},
	}
}

func newSkillsEnableCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a skill (removes it from config.skills.disabled)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return skillsPersistActiveWithOut(app, args[0], true, cmd.ErrOrStderr())
		},
	}
}

func newSkillsDisableCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a skill (adds it to config.skills.disabled)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return skillsPersistActiveWithOut(app, args[0], false, cmd.ErrOrStderr())
		},
	}
}

// loadSkills walks the skills home and applies config-driven disablement.
// Errors during skill parsing are logged to stderr but do not fail the
// command — a missing home directory is expected on fresh installs.
func loadSkills(app *App) (*skills.Registry, error) {
	reg := skills.NewRegistry()
	l := skills.NewLoader(skills.DefaultHome())
	all, errs := l.Load()
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "skills: %s: %v\n", e.Path, e.Err)
	}
	for _, s := range all {
		reg.Add(s)
	}
	if app != nil && app.Config != nil {
		// "" = no platform override; CLI-specific startup paths
		// (REPL, gateway, cron) can call ApplyConfig themselves with
		// a non-empty platform name.
		reg.ApplyConfig(skills.DisabledForPlatform(app.Config.Skills, ""))
	}
	return reg, nil
}

// skillsPersistActive updates the config file to reflect a skill's
// new enable/disable state. enable=true removes the skill from
// config.skills.disabled; enable=false adds it.
//
// Returns an error if the skill is unknown (no SKILL.md under the
// skills home directory matches).
func skillsPersistActive(app *App, name string, enable bool) error {
	return skillsPersistActiveWithOut(app, name, enable, os.Stderr)
}

// skillsPersistActiveWithOut is the io.Writer-injectable variant used
// by tests. Non-test callers should use skillsPersistActive.
func skillsPersistActiveWithOut(app *App, name string, enable bool, stderr io.Writer) error {
	reg, _ := loadSkills(app)
	if reg.Get(name) == nil {
		return fmt.Errorf("skills: unknown skill %q (not found under %s)", name, skills.DefaultHome())
	}

	app.Config.Skills = skills.WithDisabledUpdate(app.Config.Skills, name, "", !enable)

	if err := config.SaveToPath(app.ConfigPath, app.Config); err != nil {
		return fmt.Errorf("skills: persist config: %w", err)
	}

	action := "enabled"
	if !enable {
		action = "disabled"
	}
	fmt.Fprintf(stderr, "skills: %s %s (saved to %s)\n", name, action, app.ConfigPath)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cli/ -run TestSkillsEnableDisable_RoundTrip -v`
Expected: PASS

- [ ] **Step 5: Run broader skills/cli tests to confirm no regressions**

Run: `go test ./cli/... ./skills/... ./config/...`
Expected: PASS across all three packages.

- [ ] **Step 6: Commit**

```bash
git add cli/skills.go cli/skills_test.go
git commit -m "feat(cli): persist skills enable/disable to config.yaml"
```

---

## Task 7: Apply SkillsConfig at REPL startup

**Files:**
- Modify: `cli/repl.go` — find where `loadSkills` is already called, or add a call before the Engine is built
- Test: `cli/repl_test.go` (existing)

- [ ] **Step 1: Locate the skills load point**

Run: `grep -n "loadSkills\|skills.NewLoader\|skills.Registry" cli/repl.go`

Expected: there should already be a call (or a place to add one). If `loadSkills` is not called anywhere in `cli/repl.go`, the REPL currently ignores skills entirely.

- [ ] **Step 2: Write the failing test**

Append to `cli/repl_test.go`:

```go
func TestREPL_RespectsDisabledSkills(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Seed two skills.
	for _, n := range []string{"keep", "drop"} {
		p := filepath.Join(dir, "skills", "cat", n)
		_ = os.MkdirAll(p, 0o755)
		body := "---\nname: " + n + "\ndescription: t\n---\nbody"
		_ = os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644)
	}
	// Seed config disabling "drop".
	cfgPath := filepath.Join(dir, "config.yaml")
	cfgYAML := []byte("model: test\nskills:\n  disabled: [drop]\n")
	_ = os.WriteFile(cfgPath, cfgYAML, 0o644)

	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	reg, _ := loadSkills(app)
	active := reg.Active()
	if len(active) != 1 || active[0].Name != "keep" {
		t.Errorf("active = %v, want [keep]", active)
	}
}
```

Also add the needed imports at the top of `cli/repl_test.go` if not already present:
`"os"`, `"path/filepath"`, `"testing"`.

- [ ] **Step 3: Run test to verify it fails OR passes**

Run: `go test ./cli/ -run TestREPL_RespectsDisabledSkills -v`

If it already passes (Task 6's `loadSkills(app)` wire-up is sufficient), skip Step 4 and commit.

If it fails because REPL is calling the old `loadSkills()` signature (zero args) somewhere, fix the call site in `cli/repl.go` to pass `app`.

- [ ] **Step 4: (If needed) Update REPL call sites**

Find and fix call sites: `grep -n "loadSkills()" cli/` and change each to `loadSkills(app)`. Make sure `app` is in scope at each call site; thread it through if necessary.

- [ ] **Step 5: Re-run the test**

Run: `go test ./cli/ -run TestREPL_RespectsDisabledSkills -v`
Expected: PASS

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add cli/repl.go cli/repl_test.go
git commit -m "feat(cli): REPL honors config.skills.disabled at startup"
```

---

## Task 8: Manual end-to-end smoke test + docs

**Files:**
- Modify: `TODOS.md` — check off the "skills persistence" entry if present

- [ ] **Step 1: Build the binary**

Run: `go build -o /tmp/hermind ./cmd/hermind`
Expected: no errors.

- [ ] **Step 2: Set up an isolated home**

```bash
rm -rf /tmp/hermind-home
export HERMIND_HOME=/tmp/hermind-home
mkdir -p "$HERMIND_HOME/skills/demo/foo"
cat > "$HERMIND_HOME/skills/demo/foo/SKILL.md" <<'EOF'
---
name: foo
description: demo skill for smoke test
---
# foo body
EOF
mkdir -p "$HERMIND_HOME/skills/demo/bar"
cat > "$HERMIND_HOME/skills/demo/bar/SKILL.md" <<'EOF'
---
name: bar
description: demo skill number two
---
# bar body
EOF
printf 'model: anthropic/claude-opus-4-6\n' > "$HERMIND_HOME/config.yaml"
```

- [ ] **Step 3: Exercise the commands**

```bash
/tmp/hermind skills list
# expect two rows, both marked ● (active)

/tmp/hermind skills disable foo
# expect: "skills: foo disabled (saved to /tmp/hermind-home/config.yaml)"

grep -A2 '^skills:' "$HERMIND_HOME/config.yaml"
# expect disabled: [foo]

/tmp/hermind skills list
# expect foo with ○, bar with ●

/tmp/hermind skills enable foo
/tmp/hermind skills list
# expect both ● again
```

- [ ] **Step 4: Confirm error path**

```bash
/tmp/hermind skills disable ghost
# expect non-zero exit and message "unknown skill"
```

- [ ] **Step 5: Clean up**

```bash
unset HERMIND_HOME
rm -rf /tmp/hermind-home /tmp/hermind
```

- [ ] **Step 6: Note completion**

If `TODOS.md` has an entry like `- [ ] skills persistence`, check it off. Otherwise skip.

- [ ] **Step 7: Final commit**

```bash
git add -A
git commit --allow-empty -m "test(skills): manual smoke test verified enable/disable persistence"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - Python `skills.disabled` ↔ Task 1 (`Disabled` field) ✓
   - Python `skills.platform_disabled` ↔ Task 1 + Task 3 (map field + `DisabledForPlatform`) ✓
   - YAML round-trip ↔ Task 2 (`SaveToPath` atomic writer) ✓
   - Registry-level activation based on config ↔ Task 4 (`ApplyConfig`) ✓
   - CLI `enable/disable` actually writes config ↔ Task 6 ✓
   - Startup path honors config ↔ Task 7 ✓
   - Unknown skill error path ↔ Task 6 (`skills: unknown skill`) ✓

2. **Placeholders:** none — every step shows the actual code or command.

3. **Type consistency:**
   - `SkillsConfig` field name used in Task 1, 2, 3, 6, 7 — consistent.
   - `WithDisabledUpdate(cfg, name, platform, disabled)` signature — consistent across Task 3 and Task 6.
   - `skillsPersistActive(app *App, name string, enable bool)` — consistent in Task 6 + Task 7.
   - `ApplyConfig([]string)` — consistent in Task 4 + Task 6.

4. **Gaps:** the platform-specific override path (`platform_disabled`) is tested in helpers (Task 3) but no CLI flag wires a platform filter — this is deliberate MVP scope, leaving `--platform` for a follow-up plan.

---

## Definition of Done

- `go test ./config/... ./skills/... ./cli/...` all pass.
- `hermind skills list` shows an active/disabled marker per skill.
- `hermind skills disable <name>` then `hermind skills list` reflects the change.
- Re-running `hermind skills list` after restart (new process) still reflects the change.
- `~/.hermind/config.yaml` contains a `skills.disabled` block after disable.
