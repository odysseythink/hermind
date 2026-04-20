# Web Config Schema Infrastructure (Stage 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the five "simple" config sections that plug into the Stage 2 infrastructure — Logging, Metrics, Tracing, Agent, Terminal — so every non-gateway group except Models, Memory, Skills, and Advanced has a real editor instead of a "coming soon" panel.

**Architecture:** Every section is a Go `init()` in `config/descriptor/` that calls `Register` plus one row in `web/src/shell/sections.ts`. One small frontend fix is also required — see the **Discovered-during-planning bug** note below. Beyond that, `redactSecrets`, `preserveSecrets`, `handleConfigSchema`, and the generic `ConfigSection` renderer all walk `descriptor.All()` / the schema response and already support every `FieldKind` in play (string, int, bool, enum, secret; float is registered but unused here).

**Discovered-during-planning bug (fixed in Task 3):** `ConfigSection.isVisible` compares `value[f.visible_when.field] === f.visible_when.equals` with strict `===`. After boot the backend delivers `tracing.enabled` as the JSON bool `true`, but `BoolToggle.onChange` dispatches the string `"true"/"false"` (see `web/src/components/fields/BoolToggle.tsx:11`). That type drift silently breaks bool-gated visibility the moment a user toggles the switch. Stage 2 never hit this because Storage only gates on strings. Task 3 adds a coercing comparison so Tracing's `{ field: "enabled", equals: true }` predicate works on load AND after edit. This is the only infrastructure change Stage 3 needs; every other section plugs into existing plumbing.

**Tech Stack:** Go (`config/descriptor`), TypeScript + React (`web/src/shell/sections.ts`), Vitest, Go `testing`.

**Scope note — Model is deferred.** The Stage 2 footnote grouped `model` with the simple sections, but `config.Config.Model` is a top-level YAML scalar (`model: "anthropic/claude-opus-4-6"`), not a map. The ContentPanel reads `props.config[section.key]` as `Record<string, unknown>` (`web/src/components/shell/ContentPanel.tsx:53`), and `preserveSectionSecrets` reads `updM[sec.Key].(map[string]any)`. A scalar-shape section would need new infrastructure on both sides, which contradicts the Stage 2 promise of "no further infrastructure work required." It is therefore carried into Stage 4 alongside `providers` / `fallback_providers`, which is already the natural home per `web/src/shell/groups.ts` (models group `plannedStage: '3 & 4'`).

**Other deferrals recorded in the plan:**

- `agent.compression` — nested `CompressionConfig`. Representing it as a separate `compression` descriptor would break YAML round-trip because its YAML path is `agent.compression`, not top-level. Skip for Stage 3.
- `terminal.docker_volumes` — `[]string`; no matching `FieldKind`. Exposed via CLI only.
- `agent.auxiliary` — `AuxiliaryConfig` is already listed under the Runtime group's `configKeys` but is parallel in shape to `ProviderConfig` and belongs with the Stage 4 providers editor.

---

## File Structure

**Create (Go descriptors — one file per section):**

- `config/descriptor/logging.go` — one enum field (`level`).
- `config/descriptor/metrics.go` — one string field (`addr`).
- `config/descriptor/tracing.go` — one bool (`enabled`) plus one string (`file`) gated on `enabled=true`.
- `config/descriptor/agent.go` — two int fields (`max_turns`, `gateway_timeout`). Compression deferred.
- `config/descriptor/terminal.go` — one enum (`backend`) plus ten backend-specific fields (two shared, three SSH, two Modal, two Daytona, one Singularity, one Docker) guarded by `VisibleWhen`.

**Create (Go tests — matching the Storage exemplar):**

- `config/descriptor/logging_test.go`
- `config/descriptor/metrics_test.go`
- `config/descriptor/tracing_test.go`
- `config/descriptor/agent_test.go`
- `config/descriptor/terminal_test.go`

**Modify:**

- `web/src/shell/sections.ts` — append five rows to `SECTIONS`, each with `plannedStage: 'done'`.
- `web/src/shell/sections.test.ts` — extend expectations to cover every new section.
- `docs/smoke/web-config.md` — append a `## Stage 3 · Simple sections` section.
- `api/webroot/` — rebuilt from `web/dist/` by `make web-check` (committed alongside).

---

## Task 1: Logging descriptor

**Files:**

- Create: `config/descriptor/logging.go`
- Create: `config/descriptor/logging_test.go`

- [ ] **Step 1: Write the failing test**

`config/descriptor/logging_test.go`:

```go
package descriptor

import "testing"

func TestLoggingSectionRegistered(t *testing.T) {
	s, ok := Get("logging")
	if !ok {
		t.Fatal(`Get("logging") returned ok=false — did logging.go init() register?`)
	}
	if s.GroupID != "observability" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "observability")
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
	if len(s.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1", len(s.Fields))
	}
	f := s.Fields[0]
	if f.Name != "level" {
		t.Errorf("Fields[0].Name = %q, want %q", f.Name, "level")
	}
	if f.Kind != FieldEnum {
		t.Errorf("Fields[0].Kind = %s, want enum", f.Kind)
	}
	want := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	for _, v := range f.Enum {
		delete(want, v)
	}
	if len(want) > 0 {
		t.Errorf("level.Enum missing %v, got %v", want, f.Enum)
	}
	if f.Default != "info" {
		t.Errorf("level.Default = %v, want \"info\"", f.Default)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/descriptor/... -run TestLoggingSectionRegistered`

Expected: FAIL with `Get("logging") returned ok=false`.

- [ ] **Step 3: Write minimal implementation**

`config/descriptor/logging.go`:

```go
package descriptor

func init() {
	Register(Section{
		Key:     "logging",
		Label:   "Logging",
		Summary: "slog output level for the hermind process.",
		GroupID: "observability",
		Fields: []FieldSpec{
			{
				Name:     "level",
				Label:    "Level",
				Help:     "Minimum log level emitted to stderr.",
				Kind:     FieldEnum,
				Required: false,
				Default:  "info",
				Enum:     []string{"debug", "info", "warn", "error"},
			},
		},
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/descriptor/... -run TestLogging`

Expected: PASS. Also confirm `TestSectionInvariants` still passes (`go test ./config/descriptor/...`).

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/logging.go config/descriptor/logging_test.go
git commit -m "feat(config/descriptor): Logging section with single level enum"
```

---

## Task 2: Metrics descriptor

**Files:**

- Create: `config/descriptor/metrics.go`
- Create: `config/descriptor/metrics_test.go`

- [ ] **Step 1: Write the failing test**

`config/descriptor/metrics_test.go`:

```go
package descriptor

import "testing"

func TestMetricsSectionRegistered(t *testing.T) {
	s, ok := Get("metrics")
	if !ok {
		t.Fatal(`Get("metrics") returned ok=false — did metrics.go init() register?`)
	}
	if s.GroupID != "observability" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "observability")
	}
	if len(s.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1", len(s.Fields))
	}
	f := s.Fields[0]
	if f.Name != "addr" {
		t.Errorf("Fields[0].Name = %q, want %q", f.Name, "addr")
	}
	if f.Kind != FieldString {
		t.Errorf("Fields[0].Kind = %s, want string", f.Kind)
	}
	if f.Required {
		t.Error("addr.Required = true, want false (empty disables metrics)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/descriptor/... -run TestMetricsSectionRegistered`

Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

`config/descriptor/metrics.go`:

```go
package descriptor

func init() {
	Register(Section{
		Key:     "metrics",
		Label:   "Metrics",
		Summary: "Prometheus /metrics HTTP server address. Leave blank to disable.",
		GroupID: "observability",
		Fields: []FieldSpec{
			{
				Name:  "addr",
				Label: "Listen address",
				Help:  `Host:port for the exporter, e.g. ":9100". Empty disables metrics.`,
				Kind:  FieldString,
			},
		},
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/descriptor/...`

Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/metrics.go config/descriptor/metrics_test.go
git commit -m "feat(config/descriptor): Metrics section with listen addr"
```

---

## Task 3: Fix `isVisible` so bool predicates survive user edits

`ConfigSection.isVisible` currently does `===` against the predicate's `equals` value. Boolean config fields arrive from the backend as real booleans but round-trip back through `BoolToggle.onChange` as the strings `"true"/"false"`, so a predicate `{ field: "enabled", equals: true }` stops matching the instant the user flips the toggle. Coerce both sides to string before comparing. This keeps `equals: "sqlite"` (Storage) working unchanged while also handling bool, int, and float predicates — all of which share the same string-coerced dispatch path.

**Files:**

- Modify: `web/src/components/ConfigSection.tsx` — change the `isVisible` comparison.
- Modify: `web/src/components/ConfigSection.test.tsx` — add a failing regression test.

- [ ] **Step 1: Write the failing test**

Append (do not replace — the existing `describe('ConfigSection', …)` block and its imports stay as-is) this block to the END of `web/src/components/ConfigSection.test.tsx`. Every symbol used here (`describe`, `it`, `expect`, `vi`, `render`, `screen`, `userEvent`, `ConfigSection`, `ConfigSectionT`) is already imported at the top of the file by the Storage tests.

```tsx
const tracing: ConfigSectionT = {
  key: 'tracing',
  label: 'Tracing',
  group_id: 'observability',
  fields: [
    { name: 'enabled', label: 'Enabled', kind: 'bool' },
    {
      name: 'file',
      label: 'File',
      kind: 'string',
      visible_when: { field: 'enabled', equals: true },
    },
  ],
};

describe('ConfigSection isVisible — bool predicate round-trip', () => {
  it('shows the File field when enabled is the backend bool true', () => {
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: true }}
        originalValue={{ enabled: true }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/^File$/)).toBeInTheDocument();
  });

  it('shows the File field after the BoolToggle stores "true" (string)', () => {
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: 'true' }}
        originalValue={{ enabled: true }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/^File$/)).toBeInTheDocument();
  });

  it('hides the File field when enabled is "false" (string)', () => {
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: 'false' }}
        originalValue={{ enabled: true }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText(/^File$/)).toBeNull();
  });

  it('dispatches the edited enabled value as a string', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: true }}
        originalValue={{ enabled: true }}
        onFieldChange={onFieldChange}
      />,
    );
    await user.click(screen.getByLabelText(/Enabled/));
    expect(onFieldChange).toHaveBeenCalledWith('enabled', 'false');
  });
});
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `cd web && pnpm vitest run src/components/ConfigSection.test.tsx`

Expected: the second case ("shows the File field after the BoolToggle stores \"true\"") FAILS — strict `===` against the JSON-true predicate rejects the string `"true"`.

- [ ] **Step 3: Write the minimal implementation**

Edit `web/src/components/ConfigSection.tsx`. Replace the `isVisible` helper (currently at the bottom of the file) with:

```tsx
function isVisible(f: ConfigField, value: Record<string, unknown>): boolean {
  if (!f.visible_when) return true;
  // Values arrive as real types on boot (bool true, number 42) but
  // edited values pass through string-coerced field onChange handlers.
  // Coerce both sides to string so predicates keep matching either way.
  return String(value[f.visible_when.field]) === String(f.visible_when.equals);
}
```

- [ ] **Step 4: Run all ConfigSection tests to verify they pass**

Run: `cd web && pnpm vitest run src/components/ConfigSection.test.tsx`

Expected: PASS (the four new cases + the pre-existing ones).

- [ ] **Step 5: Run the full frontend suite to confirm no regression**

Run: `cd web && pnpm test`

Expected: PASS (no prior test depended on the old strict semantics — Storage predicates compare two strings, which stringify to themselves).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/ConfigSection.tsx web/src/components/ConfigSection.test.tsx
git commit -m "fix(web): ConfigSection.isVisible coerces bool/int predicates to string"
```

---

## Task 4: Tracing descriptor

**Files:**

- Create: `config/descriptor/tracing.go`
- Create: `config/descriptor/tracing_test.go`

- [ ] **Step 1: Write the failing test**

`config/descriptor/tracing_test.go`:

```go
package descriptor

import "testing"

func TestTracingSectionRegistered(t *testing.T) {
	s, ok := Get("tracing")
	if !ok {
		t.Fatal(`Get("tracing") returned ok=false`)
	}
	if s.GroupID != "observability" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "observability")
	}
	names := map[string]FieldSpec{}
	for _, f := range s.Fields {
		names[f.Name] = f
	}
	enabled, ok := names["enabled"]
	if !ok {
		t.Fatal("missing enabled field")
	}
	if enabled.Kind != FieldBool {
		t.Errorf("enabled.Kind = %s, want bool", enabled.Kind)
	}
	file, ok := names["file"]
	if !ok {
		t.Fatal("missing file field")
	}
	if file.Kind != FieldString {
		t.Errorf("file.Kind = %s, want string", file.Kind)
	}
	if file.VisibleWhen == nil {
		t.Fatal("file.VisibleWhen is nil")
	}
	if file.VisibleWhen.Field != "enabled" {
		t.Errorf("file.VisibleWhen.Field = %q, want \"enabled\"", file.VisibleWhen.Field)
	}
	if file.VisibleWhen.Equals != true {
		t.Errorf("file.VisibleWhen.Equals = %v, want true", file.VisibleWhen.Equals)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/descriptor/... -run TestTracingSectionRegistered`

Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

`config/descriptor/tracing.go`:

```go
package descriptor

func init() {
	Register(Section{
		Key:     "tracing",
		Label:   "Tracing",
		Summary: "Stdlib-based tracing emitted to a JSON-lines sink.",
		GroupID: "observability",
		Fields: []FieldSpec{
			{
				Name:    "enabled",
				Label:   "Enabled",
				Help:    "Turn tracing on.",
				Kind:    FieldBool,
				Default: false,
			},
			{
				Name:        "file",
				Label:       "File",
				Help:        "Path to the JSON-lines trace file. Leave blank for stderr.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "enabled", Equals: true},
			},
		},
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/descriptor/...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/tracing.go config/descriptor/tracing_test.go
git commit -m "feat(config/descriptor): Tracing section with enabled toggle + file path"
```

---

## Task 5: Agent descriptor

**Files:**

- Create: `config/descriptor/agent.go`
- Create: `config/descriptor/agent_test.go`

- [ ] **Step 1: Write the failing test**

`config/descriptor/agent_test.go`:

```go
package descriptor

import "testing"

func TestAgentSectionRegistered(t *testing.T) {
	s, ok := Get("agent")
	if !ok {
		t.Fatal(`Get("agent") returned ok=false`)
	}
	if s.GroupID != "runtime" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "runtime")
	}
	want := map[string]FieldKind{
		"max_turns":       FieldInt,
		"gateway_timeout": FieldInt,
	}
	got := map[string]FieldKind{}
	for _, f := range s.Fields {
		got[f.Name] = f.Kind
	}
	for name, kind := range want {
		k, ok := got[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if k != kind {
			t.Errorf("field %q: Kind = %s, want %s", name, k, kind)
		}
	}
}

func TestAgentDefaultsMatchConfigDefaults(t *testing.T) {
	s, _ := Get("agent")
	var maxTurns, gwTimeout *FieldSpec
	for i := range s.Fields {
		switch s.Fields[i].Name {
		case "max_turns":
			maxTurns = &s.Fields[i]
		case "gateway_timeout":
			gwTimeout = &s.Fields[i]
		}
	}
	if maxTurns == nil || gwTimeout == nil {
		t.Fatalf("max_turns=%v gateway_timeout=%v", maxTurns, gwTimeout)
	}
	// These mirror config.Default() so the editor's placeholder matches
	// hermind's actual runtime default when the YAML omits the key.
	if maxTurns.Default != 90 {
		t.Errorf("max_turns.Default = %v, want 90", maxTurns.Default)
	}
	if gwTimeout.Default != 1800 {
		t.Errorf("gateway_timeout.Default = %v, want 1800", gwTimeout.Default)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/descriptor/... -run TestAgentSectionRegistered`

Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

`config/descriptor/agent.go`:

```go
package descriptor

// The Agent section exposes the two top-level scalars on config.AgentConfig.
// config.AgentConfig.Compression is a nested struct and is deferred until
// the descriptor model supports nested sections; for now it is only
// editable via the CLI.
func init() {
	Register(Section{
		Key:     "agent",
		Label:   "Agent",
		Summary: "Engine turn limit and gateway request budget.",
		GroupID: "runtime",
		Fields: []FieldSpec{
			{
				Name:     "max_turns",
				Label:    "Max turns",
				Help:     "Maximum model turns per user request before the engine bails out.",
				Kind:     FieldInt,
				Required: true,
				Default:  90,
			},
			{
				Name:    "gateway_timeout",
				Label:   "Gateway timeout (seconds)",
				Help:    "Seconds a gateway request may run before being cancelled. 0 uses the gateway default.",
				Kind:    FieldInt,
				Default: 1800,
			},
		},
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/descriptor/...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/agent.go config/descriptor/agent_test.go
git commit -m "feat(config/descriptor): Agent section — max_turns + gateway_timeout"
```

---

## Task 6: Terminal descriptor

**Files:**

- Create: `config/descriptor/terminal.go`
- Create: `config/descriptor/terminal_test.go`

This is the largest section. The `backend` enum gates five subgroups of backend-specific fields; `docker_volumes` is `[]string` and therefore excluded (documented in the comment block).

- [ ] **Step 1: Write the failing test**

`config/descriptor/terminal_test.go`:

```go
package descriptor

import "testing"

func TestTerminalSectionRegistered(t *testing.T) {
	s, ok := Get("terminal")
	if !ok {
		t.Fatal(`Get("terminal") returned ok=false`)
	}
	if s.GroupID != "runtime" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "runtime")
	}

	wantKinds := map[string]FieldKind{
		"backend":           FieldEnum,
		"cwd":               FieldString,
		"timeout":           FieldInt,
		"docker_image":      FieldString,
		"ssh_host":          FieldString,
		"ssh_user":          FieldString,
		"ssh_key":           FieldString,
		"modal_base_url":    FieldString,
		"modal_token":       FieldSecret,
		"daytona_base_url":  FieldString,
		"daytona_token":     FieldSecret,
		"singularity_image": FieldString,
	}
	gotKinds := map[string]FieldKind{}
	for _, f := range s.Fields {
		gotKinds[f.Name] = f.Kind
	}
	for name, kind := range wantKinds {
		got, ok := gotKinds[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if got != kind {
			t.Errorf("field %q: Kind = %s, want %s", name, got, kind)
		}
	}
}

func TestTerminalBackendIsEnumWithSixChoices(t *testing.T) {
	s, _ := Get("terminal")
	var backend *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "backend" {
			backend = &s.Fields[i]
			break
		}
	}
	if backend == nil {
		t.Fatal("backend field not found")
	}
	if backend.Kind != FieldEnum {
		t.Fatalf("backend.Kind = %s, want enum", backend.Kind)
	}
	if backend.Default != "local" {
		t.Errorf("backend.Default = %v, want \"local\"", backend.Default)
	}
	want := map[string]bool{
		"local":       true,
		"docker":      true,
		"ssh":         true,
		"modal":       true,
		"daytona":     true,
		"singularity": true,
	}
	for _, v := range backend.Enum {
		delete(want, v)
	}
	if len(want) > 0 {
		t.Errorf("backend.Enum missing %v, got %v", want, backend.Enum)
	}
}

func TestTerminalBackendGating(t *testing.T) {
	s, _ := Get("terminal")
	gate := map[string]string{
		"docker_image":      "docker",
		"ssh_host":          "ssh",
		"ssh_user":          "ssh",
		"ssh_key":           "ssh",
		"modal_base_url":    "modal",
		"modal_token":       "modal",
		"daytona_base_url":  "daytona",
		"daytona_token":     "daytona",
		"singularity_image": "singularity",
	}
	for _, f := range s.Fields {
		want, gated := gate[f.Name]
		if !gated {
			continue
		}
		if f.VisibleWhen == nil {
			t.Errorf("field %q: VisibleWhen is nil", f.Name)
			continue
		}
		if f.VisibleWhen.Field != "backend" {
			t.Errorf("field %q: VisibleWhen.Field = %q, want \"backend\"", f.Name, f.VisibleWhen.Field)
		}
		if f.VisibleWhen.Equals != want {
			t.Errorf("field %q: VisibleWhen.Equals = %v, want %q", f.Name, f.VisibleWhen.Equals, want)
		}
	}
}

func TestTerminalSharedFieldsAreAlwaysVisible(t *testing.T) {
	s, _ := Get("terminal")
	shared := map[string]bool{"cwd": true, "timeout": true}
	for _, f := range s.Fields {
		if !shared[f.Name] {
			continue
		}
		if f.VisibleWhen != nil {
			t.Errorf("field %q: VisibleWhen = %+v, want nil (shared across backends)", f.Name, f.VisibleWhen)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/descriptor/... -run TestTerminal`

Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

`config/descriptor/terminal.go`:

```go
package descriptor

// Terminal mirrors config.TerminalConfig. docker_volumes is []string and
// has no scalar-field representation; it is left CLI-only until the
// descriptor model supports list fields.
func init() {
	Register(Section{
		Key:     "terminal",
		Label:   "Terminal",
		Summary: "Shell-exec backend used by the agent's bash tool.",
		GroupID: "runtime",
		Fields: []FieldSpec{
			{
				Name:     "backend",
				Label:    "Backend",
				Help:     "Which execution backend to run shell commands through.",
				Kind:     FieldEnum,
				Required: true,
				Default:  "local",
				Enum:     []string{"local", "docker", "ssh", "modal", "daytona", "singularity"},
			},

			// Shared across every backend.
			{
				Name:  "cwd",
				Label: "Working directory",
				Help:  "Absolute path used as cwd for each command. Empty = backend default.",
				Kind:  FieldString,
			},
			{
				Name:  "timeout",
				Label: "Default timeout (seconds)",
				Help:  "Command timeout; 0 means use the backend default.",
				Kind:  FieldInt,
			},

			// Docker backend.
			{
				Name:        "docker_image",
				Label:       "Docker image",
				Help:        "Image name passed to `docker run`.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "docker"},
			},

			// SSH backend.
			{
				Name:        "ssh_host",
				Label:       "SSH host",
				Help:        "Hostname or IP of the target machine.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "ssh"},
			},
			{
				Name:        "ssh_user",
				Label:       "SSH user",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "ssh"},
			},
			{
				Name:        "ssh_key",
				Label:       "SSH key path",
				Help:        "Filesystem path to the private key file.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "ssh"},
			},

			// Modal backend.
			{
				Name:        "modal_base_url",
				Label:       "Modal base URL",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "modal"},
			},
			{
				Name:        "modal_token",
				Label:       "Modal token",
				Kind:        FieldSecret,
				VisibleWhen: &Predicate{Field: "backend", Equals: "modal"},
			},

			// Daytona backend.
			{
				Name:        "daytona_base_url",
				Label:       "Daytona base URL",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "daytona"},
			},
			{
				Name:        "daytona_token",
				Label:       "Daytona token",
				Kind:        FieldSecret,
				VisibleWhen: &Predicate{Field: "backend", Equals: "daytona"},
			},

			// Singularity backend.
			{
				Name:        "singularity_image",
				Label:       "Singularity image",
				Help:        "Path to the .sif image file.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "singularity"},
			},
		},
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/descriptor/...`

Expected: PASS (Terminal tests + `TestSectionInvariants` still green — the latter checks every `VisibleWhen.Field` references a sibling, which is satisfied because `backend` is declared first).

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/terminal.go config/descriptor/terminal_test.go
git commit -m "feat(config/descriptor): Terminal section with per-backend gated fields"
```

---

## Task 7: Verify API schema endpoint picks up all five sections

The `/api/config/schema` handler walks `descriptor.All()` so no handler code change is required. A single additional assertion in the existing API test file locks in the expected set, protecting future-you from an accidental Register removal.

**Files:**

- Modify: `api/handlers_config_schema_test.go` — append one test.

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_config_schema_test.go`:

```go
func TestConfigSchema_IncludesStage3Sections(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/config/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.ConfigSchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := map[string]string{
		"logging":  "observability",
		"metrics":  "observability",
		"tracing":  "observability",
		"agent":    "runtime",
		"terminal": "runtime",
	}
	got := map[string]string{}
	for _, s := range body.Sections {
		if _, tracked := want[s.Key]; tracked {
			got[s.Key] = s.GroupID
		}
	}
	for key, group := range want {
		g, ok := got[key]
		if !ok {
			t.Errorf("missing section %q", key)
			continue
		}
		if g != group {
			t.Errorf("section %q: group_id = %q, want %q", key, g, group)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/... -run TestConfigSchema_IncludesStage3Sections`

Expected: PASS — tasks 1, 2, 4, 5, 6 already registered the sections, so this assertion locks the contract but does not need a fix. (If it fails, one of those tasks is missing.)

- [ ] **Step 3: Commit**

```bash
git add api/handlers_config_schema_test.go
git commit -m "test(api): pin Stage 3 sections into /api/config/schema contract"
```

---

## Task 8: Register the five sections in the frontend sidebar

**Files:**

- Modify: `web/src/shell/sections.ts`
- Modify: `web/src/shell/sections.test.ts`

- [ ] **Step 1: Write the failing tests**

Replace the body of `web/src/shell/sections.test.ts` with:

```typescript
import { describe, it, expect } from 'vitest';
import { SECTIONS, sectionsInGroup, findSection } from './sections';
import { GROUP_IDS } from './groups';

describe('SECTIONS registry', () => {
  it('every entry references a real group id', () => {
    for (const s of SECTIONS) {
      expect(GROUP_IDS.has(s.groupId)).toBe(true);
    }
  });

  it('contains storage in runtime with plannedStage=done', () => {
    const s = findSection('storage');
    expect(s).toBeDefined();
    expect(s!.groupId).toBe('runtime');
    expect(s!.plannedStage).toBe('done');
  });

  it('registers all five Stage 3 sections as done', () => {
    const stage3 = {
      logging: 'observability',
      metrics: 'observability',
      tracing: 'observability',
      agent: 'runtime',
      terminal: 'runtime',
    };
    for (const [key, group] of Object.entries(stage3)) {
      const s = findSection(key);
      expect(s, `missing ${key}`).toBeDefined();
      expect(s!.groupId).toBe(group);
      expect(s!.plannedStage).toBe('done');
    }
  });

  it('runtime group exposes storage, agent, terminal in declaration order', () => {
    const runtime = sectionsInGroup('runtime');
    expect(runtime.map(s => s.key)).toEqual(['storage', 'agent', 'terminal']);
  });

  it('observability group exposes logging, metrics, tracing in declaration order', () => {
    const observability = sectionsInGroup('observability');
    expect(observability.map(s => s.key)).toEqual(['logging', 'metrics', 'tracing']);
  });

  it('sectionsInGroup returns [] for a group with no registered sections', () => {
    expect(sectionsInGroup('memory')).toEqual([]);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && pnpm vitest run src/shell/sections.test.ts`

Expected: the two new describe blocks fail (`missing logging`, declaration-order mismatch, etc).

- [ ] **Step 3: Write minimal implementation**

Replace `web/src/shell/sections.ts` with:

```typescript
import type { GroupId } from './groups';

export interface SectionDef {
  key: string;
  groupId: GroupId;
  /** Human-readable stage marker used by the Sidebar and ComingSoonPanel. */
  plannedStage: string;
}

// Declaration order drives the Sidebar's subsection list inside each group.
// Within a group, simpler sections come before complex ones so the visual
// weight ramps up top-to-bottom.
export const SECTIONS: readonly SectionDef[] = [
  // runtime
  { key: 'storage', groupId: 'runtime', plannedStage: 'done' },
  { key: 'agent', groupId: 'runtime', plannedStage: 'done' },
  { key: 'terminal', groupId: 'runtime', plannedStage: 'done' },
  // observability
  { key: 'logging', groupId: 'observability', plannedStage: 'done' },
  { key: 'metrics', groupId: 'observability', plannedStage: 'done' },
  { key: 'tracing', groupId: 'observability', plannedStage: 'done' },
] as const;

export function sectionsInGroup(id: GroupId): readonly SectionDef[] {
  return SECTIONS.filter(s => s.groupId === id);
}

export function findSection(key: string): SectionDef | undefined {
  return SECTIONS.find(s => s.key === key);
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && pnpm vitest run src/shell/sections.test.ts`

Expected: PASS (all existing + new tests).

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/sections.ts web/src/shell/sections.test.ts
git commit -m "feat(web/shell): register Stage 3 sections in runtime + observability"
```

---

## Task 9: Full verification gauntlet

This task runs the same checklist that closed Stage 2. Every command must pass before writing the smoke doc.

**Files:** none (run commands only).

- [ ] **Step 1: Run the Go tests**

Run: `go test ./config/descriptor/... ./api/...`

Expected: PASS (no cached fails; all new tests ran).

- [ ] **Step 2: Run the web unit tests**

Run: `cd web && pnpm test`

Expected: PASS, ~155 tests total. Stage 2 ended at 149. Task 3 adds 4 cases in `ConfigSection.test.tsx`, and Task 8 adds 2 cases in `sections.test.ts` (+6). Any other delta means an unintended regression — stop and investigate.

- [ ] **Step 3: Run type-check and lint**

Run: `cd web && pnpm type-check && pnpm lint`

Expected: both PASS with zero errors and zero warnings.

- [ ] **Step 4: Rebuild the web bundle and sync `api/webroot/`**

Run: `make web-check`

Expected: Vite build succeeds; `api/webroot/` synced; smoke assertion (whatever `web-check` runs after the copy) passes.

- [ ] **Step 5: Manual smoke (required before sign-off)**

In one shell:

```bash
./bin/hermind web --addr=127.0.0.1:9119 --no-browser
```

In a browser, open the printed URL and:

- Navigate to `#observability/logging`. Observe the Level enum defaulting to `info`; change to `debug`; TopBar shows `Save · 1 change`; Save; grep the written YAML for `logging:\n  level: debug`.
- Navigate to `#observability/metrics`. Type `:9100` into the Listen address field; Save; confirm `metrics:\n  addr: :9100` in the YAML.
- Navigate to `#observability/tracing`. Toggle **Enabled** on; the File field appears. Enter `/tmp/hermind-trace.jsonl`; Save; confirm both `enabled: true` and `file: /tmp/hermind-trace.jsonl` in the YAML. Toggle off; Save; confirm `enabled: false` and the File field disappears from the editor.
- Navigate to `#runtime/agent`. Change `max_turns` to `50`; Save; confirm `agent:\n  max_turns: 50` in the YAML.
- Navigate to `#runtime/terminal`. Change **Backend** from `local` → `ssh`; the three SSH fields appear, the Docker image field disappears. Fill `ssh_host=example.com ssh_user=root ssh_key=/tmp/id_ed25519`; Save; confirm the YAML has `terminal:\n  backend: ssh\n  ssh_host: example.com …`. Flip to `modal`; only the two Modal fields are visible; the stored `ssh_*` values remain in the YAML (preserve-secrets logic keeps `modal_token` on the next flip back).

- [ ] **Step 6: Commit `api/webroot/`**

```bash
git add api/webroot/
git commit -m "chore(web): rebuild api/webroot for Stage 3 simple sections"
```

---

## Task 10: Document the smoke flow

**Files:**

- Modify: `docs/smoke/web-config.md` — append a `## Stage 3 · Simple sections` section.

- [ ] **Step 1: Append the Stage 3 section**

Append to `docs/smoke/web-config.md` (after the Stage 2 block):

```markdown
## Stage 3 · Simple sections (Logging, Metrics, Tracing, Agent, Terminal)

- Sidebar now shows three sub-entries inside Runtime (Storage, Agent, Terminal) and three inside Observability (Logging, Metrics, Tracing). Each is clickable and routes to its own editor.
- `GET /api/config/schema` returns six sections (storage + the five new ones) sorted by key.
- **Logging:** `#observability/logging` — Level enum defaults to `info`. Change to `debug`, Save, and grep for `logging:\n  level: debug` in `config.yaml`.
- **Metrics:** `#observability/metrics` — Listen address is a plain string. `:9100` round-trips.
- **Tracing:** `#observability/tracing` — Enabled toggle gates the File field. Flipping enabled off hides File; the YAML still round-trips the stored `file` because nothing was blanked on the backend (non-secret).
- **Agent:** `#runtime/agent` — Two int inputs (`max_turns`, `gateway_timeout`). Compression is not editable here (CLI-only); the sidebar description still mentions it as a later stage.
- **Terminal:** `#runtime/terminal` — Backend enum (local, docker, ssh, modal, daytona, singularity) gates per-backend fields. `modal_token` and `daytona_token` are `FieldSecret`; GET blanks them, PUT preserves them when the submitted value is empty (same round-trip behavior as `storage.postgres_url`). `docker_volumes` is intentionally absent — edit it in `config.yaml` until list-field support lands.
```

- [ ] **Step 2: Commit**

```bash
git add docs/smoke/web-config.md
git commit -m "docs(smoke): Stage 3 simple-sections smoke flow"
```

---

## Completion checklist

Before calling Stage 3 done, verify:

- [ ] `go test ./config/descriptor/... ./api/...` — PASS
- [ ] `cd web && pnpm test` — PASS
- [ ] `cd web && pnpm type-check` — PASS
- [ ] `cd web && pnpm lint` — zero warnings
- [ ] `make web-check` — PASS
- [ ] Manual smoke from Task 9 Step 5 completed and each sub-bullet verified in the written YAML.
- [ ] `docs/smoke/web-config.md` has the new Stage 3 section.
- [ ] `git status --short` — no dangling Stage-3 files (unrelated in-progress stages may remain).

Once all boxes are checked, Stage 3 is complete. The remaining unmet items from the Stage 2 footnote roll into future stages:

- **Stage 3.5 / Stage 4:** model + providers editor (Models group). Needs either scalar-section support or a synthetic `models` descriptor whose YAML mapping is fan-out rather than 1:1.
- **Later stage:** `agent.compression` (nested descriptor), `terminal.docker_volumes` (list field), `auxiliary` (paired with providers).
