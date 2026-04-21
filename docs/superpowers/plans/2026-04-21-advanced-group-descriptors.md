# Advanced Group Descriptors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the web-UI **Advanced** group to real config editors for `browser`, `mcp`, and `cron` by extending the descriptor framework with two new concepts (section-level YAML `Subkey` wrapper + opt-in `NoDiscriminator` mode on list/keyed-map shapes), then register three descriptors that exercise both old and new shape patterns.

**Architecture:** Two small framework extensions, then three descriptors layered on top.

1. **`Section.Subkey string`** — optional YAML wrapper key. `mcp` maps to `cfg.mcp.servers.<name>.…` (subkey `servers`); `cron` maps to `cfg.cron.jobs[i].…` (subkey `jobs`). When empty, section value is read directly as before (`memory`, `providers`, etc.).
2. **`Section.NoDiscriminator bool`** — opt-in relaxation for `ShapeKeyedMap` / `ShapeList` invariants. When true, a `provider` FieldEnum is not required, and the frontend creation UX swaps the provider-picker for a name-only dialog (KeyedMap) or a blank-append button (List).
3. **Three descriptors:**
   - `browser` — `ShapeMap` with `provider` discriminator + dotted sub-fields (mirrors `memory.go`)
   - `mcp` — `ShapeKeyedMap`, `Subkey: "servers"`, `NoDiscriminator: true`; ships `command` + `enabled` only (args/env deferred; see "Deferred" below)
   - `cron` — `ShapeList`, `Subkey: "jobs"`, `NoDiscriminator: true`; four fields (name, schedule, prompt, model)

**Tech Stack:** Go (descriptor + API handler), TypeScript + React + zod (web UI), existing superpowers plan format.

**Deferred (not in scope):**
- `FieldStringList` / `FieldStringMap` kinds — needed for mcp `args`/`env`. MCP section help text will tell users to edit YAML for those. Follow-up plan.
- `FieldTextArea` multi-line kind — would help cron `prompt`. Ships as single-line FieldString for now; YAML-edit the long cases.
- Env-override surfacing (BROWSERBASE_API_KEY / BROWSERBASE_PROJECT_ID) — UI will show YAML values only. A future "env overrides this field" badge is out of scope.

---

## File Structure

**New files:**
- `config/descriptor/browser.go` — browser descriptor registration (ShapeMap)
- `config/descriptor/browser_test.go` — registration + gated-field tests
- `config/descriptor/mcp.go` — mcp descriptor (ShapeKeyedMap, Subkey, NoDiscriminator)
- `config/descriptor/mcp_test.go`
- `config/descriptor/cron.go` — cron descriptor (ShapeList, Subkey, NoDiscriminator)
- `config/descriptor/cron_test.go`
- `docs/smoke/advanced.md` — manual smoke flow (all three sections)

**Modified files:**
- `config/descriptor/descriptor.go` — add `Subkey string` + `NoDiscriminator bool` fields to `Section`
- `config/descriptor/descriptor_test.go` — relax provider-enum invariants when `NoDiscriminator: true`
- `api/handlers_config.go` — unwrap `Subkey` layer in redactSecrets + preserveSecrets
- `api/handlers_config_test.go` — round-trip test with a dummy Subkey-using descriptor
- `api/dto.go` — add `Subkey`, `NoDiscriminator` to `ConfigSectionDTO`
- `api/handlers_config_schema.go` — populate the new DTO fields
- `api/handlers_config_schema_test.go` — assert new DTO fields are emitted
- `web/src/api/schemas.ts` — extend `ConfigSectionSchema` with optional `subkey`, `no_discriminator`
- `web/src/components/shell/ContentPanel.tsx` — route ShapeKeyedMap/ShapeList through `subkey`
- `web/src/components/groups/models/NewProviderDialog.tsx` — name-only variant when `no_discriminator`
- `web/src/App.tsx` — creation flow for `no_discriminator` (KeyedMap: default-fill; List: blank append)
- `web/src/shell/groups.ts` — flip `advanced` `plannedStage: '7'` → `'done'`

---

## Scope Check

Three subsystems, but all three pivot on the same descriptor-framework extension (Subkey + NoDiscriminator). Splitting them into three plans would duplicate the framework work. Keeping as one plan.

---

### Task 1: Extend `Section` with `Subkey` + `NoDiscriminator`

**Files:**
- Modify: `config/descriptor/descriptor.go:110-117` (Section struct)
- Modify: `config/descriptor/descriptor_test.go` (invariants)

**Why:** Two new concepts on the Section type. `Subkey` lets a section's value be wrapped in one extra YAML key before the Shape-specific payload (e.g. `mcp.servers.<k>` vs `mcp.<k>`). `NoDiscriminator` opts out of the "exactly one FieldEnum named provider" requirement for ShapeKeyedMap/ShapeList — cron/mcp have uniform fields with no type discriminator.

- [ ] **Step 1: Write the failing tests**

Add to `config/descriptor/descriptor_test.go` (at end of file):

```go
func TestSectionSubkeyDefaultsToEmpty(t *testing.T) {
	Register(Section{
		Key: "t_subkey_default", Label: "x", GroupID: "runtime",
		Shape:  ShapeMap,
		Fields: []FieldSpec{{Name: "f", Label: "F", Kind: FieldString}},
	})
	t.Cleanup(func() { Unregister("t_subkey_default") })
	s, _ := Get("t_subkey_default")
	if s.Subkey != "" {
		t.Errorf("Subkey default = %q, want empty", s.Subkey)
	}
	if s.NoDiscriminator {
		t.Error("NoDiscriminator default = true, want false")
	}
}

func TestShapeKeyedMapWithNoDiscriminatorSkipsProviderRequirement(t *testing.T) {
	Register(Section{
		Key: "t_km_nodisc", Label: "x", GroupID: "runtime",
		Shape:  ShapeKeyedMap,
		Subkey: "servers",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "command", Label: "C", Kind: FieldString, Required: true},
		},
	})
	t.Cleanup(func() { Unregister("t_km_nodisc") })
	if err := validateRegistry(); err != nil {
		t.Errorf("validateRegistry rejected no-discriminator ShapeKeyedMap: %v", err)
	}
}

func TestShapeListWithNoDiscriminatorSkipsProviderRequirement(t *testing.T) {
	Register(Section{
		Key: "t_list_nodisc", Label: "x", GroupID: "runtime",
		Shape:  ShapeList,
		Subkey: "jobs",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "name", Label: "N", Kind: FieldString, Required: true},
		},
	})
	t.Cleanup(func() { Unregister("t_list_nodisc") })
	if err := validateRegistry(); err != nil {
		t.Errorf("validateRegistry rejected no-discriminator ShapeList: %v", err)
	}
}

func TestShapeKeyedMapWithoutNoDiscriminatorStillRequiresProvider(t *testing.T) {
	Register(Section{
		Key: "t_km_still_req", Label: "x", GroupID: "runtime",
		Shape:  ShapeKeyedMap,
		// NoDiscriminator NOT set
		Fields: []FieldSpec{
			{Name: "base_url", Label: "B", Kind: FieldString},
		},
	})
	t.Cleanup(func() { Unregister("t_km_still_req") })
	if err := validateRegistry(); err == nil {
		t.Error("validateRegistry accepted ShapeKeyedMap missing provider enum; want rejection")
	}
}
```

> If the existing test file already has a helper named `validateRegistry`, reuse it exactly. If the existing invariant test functions reference the validator through a different name, match that name instead (grep `config/descriptor/descriptor_test.go` for `func Test.*Invariant` and mirror the calling convention).

- [ ] **Step 2: Run tests — expect compile failure**

Run: `cd config/descriptor && go test ./...`
Expected: compile error — `s.Subkey undefined`, `s.NoDiscriminator undefined`.

- [ ] **Step 3: Add fields to `Section`**

Edit `config/descriptor/descriptor.go` Section struct (currently lines 110–117). After the `Fields` field add:

```go
	// Subkey is the YAML key that wraps the shape-specific payload.
	// Empty (default) means the section's value IS the payload (memory,
	// providers). Non-empty means the payload lives one level deeper, e.g.
	// Subkey="servers" for mcp means YAML path is cfg.mcp.servers.<k>.<f>.
	// The API redact/preserve pipeline and the UI both unwrap this layer
	// transparently.
	Subkey string
	// NoDiscriminator, when true, opts out of the "exactly one FieldEnum
	// named provider" requirement on ShapeKeyedMap / ShapeList. Used for
	// sections where every instance has the same fields (mcp servers,
	// cron jobs). Ignored on ShapeMap / ShapeScalar.
	NoDiscriminator bool
```

- [ ] **Step 4: Relax the invariant**

Find the invariant validator in `config/descriptor/descriptor_test.go` (or `descriptor.go` if there's a public `Validate()`). Locate the branch that requires a `provider` FieldEnum on ShapeKeyedMap/ShapeList — per the survey, assertions live near descriptor_test.go:140 (KeyedMap) and :155 (List). Wrap those branches with `if !s.NoDiscriminator { … }`.

Example (exact wording depends on current validator — adapt):

```go
case ShapeKeyedMap, ShapeList:
	if len(s.Fields) == 0 {
		return fmt.Errorf("section %q: %v requires at least one field", s.Key, s.Shape)
	}
	if !s.NoDiscriminator {
		var provider *FieldSpec
		for i := range s.Fields {
			if s.Fields[i].Name == "provider" && s.Fields[i].Kind == FieldEnum {
				if provider != nil {
					return fmt.Errorf("section %q: multiple provider enums", s.Key)
				}
				provider = &s.Fields[i]
			}
		}
		if provider == nil {
			return fmt.Errorf("section %q: %v requires exactly one FieldEnum named \"provider\"", s.Key, s.Shape)
		}
	}
```

- [ ] **Step 5: Run tests — expect pass**

Run: `cd config/descriptor && go test ./...`
Expected: all tests pass (the three new ones plus existing invariant tests).

- [ ] **Step 6: Commit**

```bash
git add config/descriptor/descriptor.go config/descriptor/descriptor_test.go
git commit -m "feat(descriptor): Section.Subkey + NoDiscriminator — YAML wrapper layer + opt-out of provider discriminator for KeyedMap/List"
```

---

### Task 2: Redact / preserve secrets through `Subkey`

**Files:**
- Modify: `api/handlers_config.go` (redactSecrets + preserveSecrets)
- Modify: `api/handlers_config_test.go`

**Why:** The redact path currently does `m[sec.Key].(map[string]any)` for KeyedMap and `m[sec.Key].([]any)` for List. With a non-empty `Subkey`, we need one more lookup: `m[sec.Key].(map[string]any)[sec.Subkey]` before shape-specific iteration. Same story on preserve.

- [ ] **Step 1: Write the failing round-trip test**

Add to `api/handlers_config_test.go`:

```go
func TestRedactAndPreserve_HonorsSubkey(t *testing.T) {
	// Register an ad-hoc descriptor: ShapeKeyedMap under Subkey "servers",
	// NoDiscriminator, single secret field "api_key".
	key := "t_subkey_redact"
	descriptor.Register(descriptor.Section{
		Key: key, Label: "t", GroupID: "runtime",
		Shape: descriptor.ShapeKeyedMap, Subkey: "servers",
		NoDiscriminator: true,
		Fields: []descriptor.FieldSpec{
			{Name: "api_key", Label: "k", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	original := map[string]any{
		key: map[string]any{
			"servers": map[string]any{
				"foo": map[string]any{"api_key": "real-secret-1"},
				"bar": map[string]any{"api_key": "real-secret-2"},
			},
		},
	}
	// redact in place — copy first since redactSecrets mutates
	redacted := deepCopyMap(original)
	redactSecrets(redacted)

	gotFoo := redacted[key].(map[string]any)["servers"].(map[string]any)["foo"].(map[string]any)["api_key"]
	if gotFoo != redactedPlaceholder {
		t.Errorf("foo.api_key = %v, want redacted placeholder", gotFoo)
	}

	// Now simulate PUT: user sends the redacted placeholder for foo, a real
	// new value for bar. preserveSecrets should carry foo's original back,
	// keep bar's new value.
	incoming := map[string]any{
		key: map[string]any{
			"servers": map[string]any{
				"foo": map[string]any{"api_key": redactedPlaceholder},
				"bar": map[string]any{"api_key": "new-bar-secret"},
			},
		},
	}
	preserveSecrets(incoming, original)

	gotFoo2 := incoming[key].(map[string]any)["servers"].(map[string]any)["foo"].(map[string]any)["api_key"]
	if gotFoo2 != "real-secret-1" {
		t.Errorf("after preserve: foo.api_key = %v, want real-secret-1 preserved", gotFoo2)
	}
	gotBar2 := incoming[key].(map[string]any)["servers"].(map[string]any)["bar"].(map[string]any)["api_key"]
	if gotBar2 != "new-bar-secret" {
		t.Errorf("after preserve: bar.api_key = %v, want new-bar-secret", gotBar2)
	}
}
```

> Adapt `redactedPlaceholder` and `deepCopyMap` to match existing test helpers (grep the same file for `redactedPlaceholder` and test-level deep copy helpers). If `deepCopyMap` doesn't exist, inline two levels of map copy.

- [ ] **Step 2: Run — expect failure**

Run: `cd api && go test -run TestRedactAndPreserve_HonorsSubkey`
Expected: FAIL — test accesses `["servers"]` which won't exist because redact operates at top-level keyed-map layer that isn't there (it tries to walk `cfg[key]` as KeyedMap directly).

- [ ] **Step 3: Unwrap Subkey in redactSecrets**

In `api/handlers_config.go` find the `ShapeKeyedMap` branch (around line 84). Before the `m[sec.Key].(map[string]any)` type-assert, insert a helper that walks `Subkey` if set. Do the same in the `ShapeList` branch (around line 106).

Suggested pattern — add a small helper near the top of the file:

```go
// unwrapSection returns the payload for sec from m, walking sec.Subkey
// one level deeper when non-empty. The caller still type-asserts the
// result to map[string]any or []any as per Shape. Returns nil, false
// when the path is absent or the intermediate isn't a map.
func unwrapSection(m map[string]any, sec descriptor.Section) (any, bool) {
	raw, ok := m[sec.Key]
	if !ok || raw == nil {
		return nil, false
	}
	if sec.Subkey == "" {
		return raw, true
	}
	inner, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := inner[sec.Subkey]
	if !ok {
		return nil, false
	}
	return v, true
}
```

Refactor the ShapeKeyedMap branch to:

```go
case descriptor.ShapeKeyedMap:
	raw, ok := unwrapSection(m, sec)
	if !ok { continue }
	outer, ok := raw.(map[string]any)
	if !ok { continue }
	// ... existing inner-iteration logic unchanged ...
```

Same for ShapeList — swap the `([]any)` assertion to go through `unwrapSection`.

- [ ] **Step 4: Unwrap Subkey in preserveSecrets**

preserveSecrets has the mirror structure (lines 251 / 284 per survey). Apply the same `unwrapSection` indirection on both the `updated` map and the `original` map before the shape-specific secret-copy loops.

- [ ] **Step 5: Run — expect pass**

Run: `cd api && go test -run TestRedactAndPreserve_HonorsSubkey`
Expected: PASS.

Also run the full package: `cd api && go test ./...`
Expected: no regressions.

- [ ] **Step 6: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go
git commit -m "feat(api): redact/preserve unwrap descriptor.Section.Subkey layer"
```

---

### Task 3: Expose `Subkey` + `NoDiscriminator` in schema DTO

**Files:**
- Modify: `api/dto.go` (ConfigSectionDTO)
- Modify: `api/handlers_config_schema.go`
- Modify: `api/handlers_config_schema_test.go`

**Why:** The web UI needs to know about Subkey (to unwrap one layer when reading/writing the list/map) and NoDiscriminator (to swap new-instance UX). Both travel via the existing `/api/config/schema` payload.

- [ ] **Step 1: Write the failing test**

Add to `api/handlers_config_schema_test.go`:

```go
func TestConfigSchema_EmitsSubkeyAndNoDiscriminator(t *testing.T) {
	key := "t_schema_subkey"
	descriptor.Register(descriptor.Section{
		Key: key, Label: "Subkey probe", GroupID: "runtime",
		Shape: descriptor.ShapeKeyedMap, Subkey: "servers",
		NoDiscriminator: true,
		Fields: []descriptor.FieldSpec{
			{Name: "command", Label: "C", Kind: descriptor.FieldString, Required: true},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	body := doGet(t, newTestServer(t), "/api/config/schema")
	var resp struct {
		Sections []struct {
			Key             string `json:"key"`
			Subkey          string `json:"subkey"`
			NoDiscriminator bool   `json:"no_discriminator"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var found *struct {
		Key             string `json:"key"`
		Subkey          string `json:"subkey"`
		NoDiscriminator bool   `json:"no_discriminator"`
	}
	for i := range resp.Sections {
		if resp.Sections[i].Key == key {
			found = &resp.Sections[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("section %q missing from schema response", key)
	}
	if found.Subkey != "servers" {
		t.Errorf("subkey = %q, want \"servers\"", found.Subkey)
	}
	if !found.NoDiscriminator {
		t.Error("no_discriminator = false, want true")
	}
}
```

> Adapt `doGet` / `newTestServer` to whatever helpers the existing schema test file already uses (look at the other tests in the same file).

- [ ] **Step 2: Run — expect failure**

Run: `cd api && go test -run TestConfigSchema_EmitsSubkeyAndNoDiscriminator`
Expected: FAIL — `subkey` / `no_discriminator` keys missing, assertions fail.

- [ ] **Step 3: Extend `ConfigSectionDTO`**

Edit `api/dto.go` ConfigSectionDTO (around lines 199–206). Add:

```go
	Subkey          string `json:"subkey,omitempty"`
	NoDiscriminator bool   `json:"no_discriminator,omitempty"`
```

Use `omitempty` so the JSON stays clean for descriptors that don't set these.

- [ ] **Step 4: Populate at marshaling site**

Edit `api/handlers_config_schema.go` — in the `for _, sec := range all` loop (around line 19), within the `ConfigSectionDTO{…}` literal (around line 46), add:

```go
		Subkey:          sec.Subkey,
		NoDiscriminator: sec.NoDiscriminator,
```

- [ ] **Step 5: Run — expect pass**

Run: `cd api && go test -run TestConfigSchema_EmitsSubkeyAndNoDiscriminator`
Expected: PASS.

Also re-run full package: `cd api && go test ./...`

- [ ] **Step 6: Commit**

```bash
git add api/dto.go api/handlers_config_schema.go api/handlers_config_schema_test.go
git commit -m "feat(api/schema): expose descriptor Subkey + NoDiscriminator in /api/config/schema"
```

---

### Task 4: Extend zod schema with `subkey` + `no_discriminator`

**Files:**
- Modify: `web/src/api/schemas.ts`

**Why:** TS side needs to accept the new JSON fields without tripping zod validation, and expose them via the inferred `ConfigSection` type so downstream components can read them.

- [ ] **Step 1: Write the failing test**

Grep for an existing test file next to schemas.ts. If one exists (e.g. `web/src/api/schemas.test.ts`), add a test there. If not, create it with minimal scaffolding mirroring any other `*.test.ts` in `web/src/`:

```ts
import { describe, it, expect } from 'vitest';
import { ConfigSectionSchema } from './schemas';

describe('ConfigSectionSchema', () => {
  it('accepts subkey + no_discriminator', () => {
    const parsed = ConfigSectionSchema.parse({
      key: 'mcp',
      label: 'MCP',
      shape: 'keyed_map',
      subkey: 'servers',
      no_discriminator: true,
      fields: [
        { name: 'command', label: 'Command', kind: 'string' },
      ],
    });
    expect(parsed.subkey).toBe('servers');
    expect(parsed.no_discriminator).toBe(true);
  });

  it('defaults subkey/no_discriminator to undefined when omitted', () => {
    const parsed = ConfigSectionSchema.parse({
      key: 'memory',
      label: 'Memory',
      shape: 'map',
      fields: [],
    });
    expect(parsed.subkey).toBeUndefined();
    expect(parsed.no_discriminator).toBeUndefined();
  });
});
```

- [ ] **Step 2: Run — expect failure**

Run: `cd web && npm test -- schemas`
Expected: FAIL — the first test's `parse` throws (unrecognized keys) unless the schema is extended.

- [ ] **Step 3: Extend the schema**

In `web/src/api/schemas.ts` find `ConfigSectionSchema` (around lines 104–112 per survey). Add inside the zod object:

```ts
  subkey: z.string().optional(),
  no_discriminator: z.boolean().optional(),
```

- [ ] **Step 4: Run — expect pass**

Run: `cd web && npm test -- schemas`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/api/schemas.ts web/src/api/schemas.test.ts
git commit -m "feat(web/schema): accept subkey + no_discriminator on ConfigSection"
```

> If the test file already existed before this task, omit it from `git add` and just commit `schemas.ts` + any additions.

---

### Task 5: Route `ShapeKeyedMap` / `ShapeList` through `subkey` in the UI

**Files:**
- Modify: `web/src/components/shell/ContentPanel.tsx`

**Why:** The routing layer currently reads `config[section.key]` and hands it to editor components. With `subkey`, we must pass `config[section.key][section.subkey]` (and ensure writes target the same nested path). The editor components themselves (`ProviderEditor`, `FallbackProviderEditor`) don't need to know — they already get a single instance/element and call `onFieldChange(name, value)` which is where we wire the dotted-path indirection.

> **Context for the implementer:** In the current code path, `ContentPanel` selects a section, then derives the current instance/element from `config[section.key]`, then passes the instance to the editor. When `subkey` is set, the instance list lives at `config[section.key][subkey]` instead. The field-change reducer already supports dotted names via `setPath`, so we can just prefix with `${subkey}.` when constructing the onChange, OR we can compose reads at the ContentPanel level and writes at the App/state level. The simpler approach: make ContentPanel compute `instances = subkey ? config[sec.key]?.[subkey] : config[sec.key]` for display, and wrap the per-section `onFieldChange` to prepend the subkey.

- [ ] **Step 1: Read the relevant ContentPanel region**

Read `web/src/components/shell/ContentPanel.tsx` lines 80–200 and locate:
- where `shape === 'keyed_map'` / `shape === 'list'` is handled (ContentPanel.tsx:90 per survey)
- where it derives the instance list from `config`
- where it passes `onFieldChange` down to the editor

Also read the editor signatures (`ProviderEditor`, `FallbackProviderEditor`) briefly so you understand the onFieldChange contract.

- [ ] **Step 2: Write a component test first**

If a test file exists at `web/src/components/shell/ContentPanel.test.tsx`, add the test case there. Scaffold for a new case:

```ts
it('reads instances from config[key][subkey] when subkey is set', () => {
  const section = {
    key: 'mcp', label: 'MCP', shape: 'keyed_map',
    subkey: 'servers', no_discriminator: true,
    fields: [{ name: 'command', label: 'Command', kind: 'string' }],
  };
  const config = {
    mcp: { servers: { foo: { command: '/bin/foo' } } },
  };
  const result = render(<ContentPanel section={section} config={config}
    activeSubKey="foo" onFieldChange={vi.fn()} originalValue={config} />);
  expect(result.getByDisplayValue('/bin/foo')).toBeTruthy();
});
```

> Adapt props to ContentPanel's actual signature; look at neighboring tests in the file for the correct shape.

- [ ] **Step 3: Run — expect failure**

Run: `cd web && npm test -- ContentPanel`
Expected: FAIL — reads `config.mcp.foo.command` (undefined) instead of `config.mcp.servers.foo.command`.

- [ ] **Step 4: Unwrap `subkey` in ContentPanel**

In the keyed_map / list handling block, replace reads of `config[section.key]` with a local helper:

```ts
const outer = config[section.key];
const instances = section.subkey
  ? (outer as Record<string, unknown> | undefined)?.[section.subkey]
  : outer;
```

And wrap the passed `onFieldChange` to include the subkey in the dotted path when calling upward. Concretely, when the section has a `subkey`, transform `onFieldChange(fieldName, value)` to `parentOnFieldChange(\`\${subkey}.\${fieldName}\`, value)` at the ContentPanel boundary — the reducer in `state.ts` will setPath through the dotted name.

> If the current implementation passes `onFieldChange` through unchanged and the reducer operates on `section.key` + `fieldName`, you may instead need to lift the subkey injection one level up into the reducer. Check state.ts before implementing; whichever layer already handles dotted names (setPath / walkPath) is the layer to inject at. Do not duplicate the logic in two places.

- [ ] **Step 5: Run — expect pass**

Run: `cd web && npm test -- ContentPanel`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shell/ContentPanel.tsx web/src/components/shell/ContentPanel.test.tsx
git commit -m "feat(web/shell): ContentPanel unwraps section.subkey for keyed_map/list reads + writes"
```

---

### Task 6: Name-only new-instance UX when `no_discriminator` is set

**Files:**
- Modify: `web/src/components/groups/models/NewProviderDialog.tsx`
- Modify: `web/src/App.tsx` (creation flow for ShapeKeyedMap and ShapeList)

**Why:** The existing NewProviderDialog prompts for both instance-key and provider-type. When NoDiscriminator is true, the provider picker is a lie (there's no discriminator), so we hide it and default the new instance to the fields' defaults. For ShapeList, App.tsx currently appends `{provider: firstType, ...}` — under NoDiscriminator, we append a blank element (with Field defaults).

- [ ] **Step 1: Write a failing dialog test**

Add to `NewProviderDialog.test.tsx` (or create):

```ts
it('hides provider picker when noDiscriminator is true', () => {
  const onCreate = vi.fn();
  const field = { name: 'command', label: 'Command', kind: 'string' };
  render(<NewProviderDialog open section={{
    key: 'mcp', shape: 'keyed_map', subkey: 'servers',
    no_discriminator: true, fields: [field],
  }} onCreate={onCreate} onClose={() => {}} />);
  // Provider select is NOT rendered
  expect(screen.queryByLabelText(/provider/i)).toBeNull();
  // Instance key input IS rendered
  expect(screen.getByLabelText(/name|instance|key/i)).toBeTruthy();
});
```

- [ ] **Step 2: Run — expect failure**

Run: `cd web && npm test -- NewProviderDialog`

- [ ] **Step 3: Update NewProviderDialog**

When `section.no_discriminator` is true:
- Skip the `<select>` that asks for provider type.
- On submit, build the new instance as `Object.fromEntries(section.fields.map(f => [f.name, f.default ?? defaultFor(f.kind)]))` where `defaultFor` returns `""` / `false` / `0` per kind.

Preserve the existing behavior (provider picker) when `no_discriminator` is falsy.

- [ ] **Step 4: Update App.tsx ShapeList creation**

Find the code in App.tsx (around lines 273–286 per survey) that creates new fallback-provider elements with `{provider: firstType, ...}`. Branch on `section.no_discriminator`:

```ts
const firstType = providerField?.enum?.[0];
const seed = section.no_discriminator
  ? Object.fromEntries(section.fields.map(f => [f.name, defaultFor(f)]))
  : { provider: firstType, /* existing fields */ };
```

Keep the existing `fallback:${list.length}` sub-key selection after append.

> `defaultFor(field)` should return a sensible zero — `""` for string/secret/enum, `false` for bool, `0` for int/float, `[]` for multiselect. Export a small helper from `web/src/util/defaults.ts` if one doesn't already exist.

- [ ] **Step 5: Write a failing test for the ShapeList append flow**

In `web/src/state.test.ts` or the relevant App-level test file, add:

```ts
it('appends a blank element to a no_discriminator ShapeList', () => {
  const section = {
    key: 'cron', shape: 'list', subkey: 'jobs', no_discriminator: true,
    fields: [
      { name: 'name', label: 'Name', kind: 'string' },
      { name: 'schedule', label: 'Schedule', kind: 'string' },
    ],
  };
  const config = { cron: { jobs: [] } };
  const next = appendListElement(config, section);  // adapt to real fn name
  expect(next.cron.jobs).toEqual([{ name: '', schedule: '' }]);
  expect(next.cron.jobs[0].provider).toBeUndefined();
});
```

- [ ] **Step 6: Run all web tests — expect pass**

Run: `cd web && npm test`
Expected: full suite green.

- [ ] **Step 7: Commit**

```bash
git add web/src/components/groups/models/NewProviderDialog.tsx web/src/App.tsx web/src/components/groups/models/NewProviderDialog.test.tsx web/src/state.test.ts
git commit -m "feat(web): name-only new-instance UX when section.no_discriminator is set"
```

---

### Task 7: Register `browser` descriptor (ShapeMap + provider discriminator)

**Files:**
- Create: `config/descriptor/browser.go`
- Create: `config/descriptor/browser_test.go`

**Why:** Browser is the simplest of the three — a near-copy of memory.go's pattern. Provider enum discriminator (`""`, `"browserbase"`, `"camofox"`) + dotted sub-fields (`browserbase.api_key` etc., `camofox.base_url` etc.) gated by `VisibleWhen`.

- [ ] **Step 1: Write the failing tests**

Create `config/descriptor/browser_test.go`:

```go
package descriptor

import "testing"

func TestBrowserSectionRegistered(t *testing.T) {
	s, ok := Get("browser")
	if !ok {
		t.Fatal("Get(\"browser\") returned ok=false — did browser.go init() register?")
	}
	if s.GroupID != "advanced" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "advanced")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
}

func TestBrowserProviderEnum(t *testing.T) {
	s, _ := Get("browser")
	var p *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "provider" {
			p = &s.Fields[i]
			break
		}
	}
	if p == nil { t.Fatal("provider field missing") }
	if p.Kind != FieldEnum { t.Errorf("Kind = %s, want enum", p.Kind) }
	want := map[string]bool{"": true, "browserbase": true, "camofox": true}
	for _, v := range p.Enum { delete(want, v) }
	if len(want) > 0 {
		t.Errorf("provider.Enum missing %v, got %v", want, p.Enum)
	}
}

func TestBrowserbaseApiKeyIsSecretGatedByProvider(t *testing.T) {
	s, _ := Get("browser")
	var f *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "browserbase.api_key" {
			f = &s.Fields[i]
			break
		}
	}
	if f == nil { t.Fatal("browserbase.api_key missing") }
	if f.Kind != FieldSecret { t.Errorf("Kind = %s, want secret", f.Kind) }
	if f.VisibleWhen == nil || f.VisibleWhen.Field != "provider" || f.VisibleWhen.Equals != "browserbase" {
		t.Errorf("VisibleWhen = %+v, want {provider=browserbase}", f.VisibleWhen)
	}
}

func TestCamofoxFieldsGatedByProvider(t *testing.T) {
	s, _ := Get("browser")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields { byName[s.Fields[i].Name] = &s.Fields[i] }
	for _, name := range []string{"camofox.base_url", "camofox.managed_persistence"} {
		f, ok := byName[name]
		if !ok { t.Errorf("field %q missing", name); continue }
		if f.VisibleWhen == nil || f.VisibleWhen.Field != "provider" || f.VisibleWhen.Equals != "camofox" {
			t.Errorf("field %q: VisibleWhen = %+v, want {provider=camofox}", name, f.VisibleWhen)
		}
	}
}
```

- [ ] **Step 2: Run — expect failure**

Run: `cd config/descriptor && go test -run TestBrowser`
Expected: FAIL — `Get("browser")` returns ok=false.

- [ ] **Step 3: Create `browser.go`**

```go
package descriptor

// Browser mirrors config.BrowserConfig. The provider field is a FieldEnum
// discriminator: when blank, no browser provider is configured (matches
// yaml omitempty). Each backend's sub-fields are gated by VisibleWhen so
// only the active backend renders.
//
// Dotted field names like "browserbase.api_key" rely on the dotted-path
// infrastructure already in ConfigSection.tsx, state.ts, and
// api/handlers_config.go (walkPath helper).
func init() {
	gate := func(backend string) *Predicate {
		return &Predicate{Field: "provider", Equals: backend}
	}
	Register(Section{
		Key:     "browser",
		Label:   "Browser",
		Summary: "Browser automation provider. Leave blank for no browser integration.",
		GroupID: "advanced",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "provider",
				Label: "Provider",
				Help:  "Browser automation backend. Leave blank to disable.",
				Kind:  FieldEnum,
				Enum:  []string{"", "browserbase", "camofox"},
			},

			// Browserbase (cloud)
			{Name: "browserbase.base_url", Label: "Browserbase base URL",
				Kind: FieldString, VisibleWhen: gate("browserbase")},
			{Name: "browserbase.api_key", Label: "Browserbase API key",
				Kind: FieldSecret, VisibleWhen: gate("browserbase"),
				Help: "Env var BROWSERBASE_API_KEY overrides this value at runtime."},
			{Name: "browserbase.project_id", Label: "Browserbase project ID",
				Kind: FieldString, VisibleWhen: gate("browserbase"),
				Help: "Env var BROWSERBASE_PROJECT_ID overrides this value at runtime."},
			{Name: "browserbase.keep_alive", Label: "Keep session alive",
				Kind: FieldBool, VisibleWhen: gate("browserbase")},
			{Name: "browserbase.proxies", Label: "Enable Browserbase proxies",
				Kind: FieldBool, VisibleWhen: gate("browserbase")},

			// Camofox (local)
			{Name: "camofox.base_url", Label: "Camofox base URL",
				Kind: FieldString, VisibleWhen: gate("camofox"),
				Help: "Defaults to http://localhost:9377 when blank."},
			{Name: "camofox.managed_persistence", Label: "Managed persistence",
				Kind: FieldBool, VisibleWhen: gate("camofox")},
		},
	})
}
```

- [ ] **Step 4: Run — expect pass**

Run: `cd config/descriptor && go test -run TestBrowser`
Expected: PASS.

Also run the whole package to catch invariant issues: `cd config/descriptor && go test ./...`

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/browser.go config/descriptor/browser_test.go
git commit -m "feat(config/descriptor): Browser section — provider discriminator with browserbase/camofox sub-fields"
```

---

### Task 8: Register `cron` descriptor (ShapeList, Subkey, NoDiscriminator)

**Files:**
- Create: `config/descriptor/cron.go`
- Create: `config/descriptor/cron_test.go`

**Why:** Cron is the first descriptor to exercise both new concepts (Subkey + NoDiscriminator). Each job is a uniform `{name, schedule, prompt, model}` — no discriminator. The YAML is `cron.jobs: [...]` so Subkey="jobs".

- [ ] **Step 1: Write failing tests**

Create `config/descriptor/cron_test.go`:

```go
package descriptor

import "testing"

func TestCronSectionRegistered(t *testing.T) {
	s, ok := Get("cron")
	if !ok { t.Fatal("Get(\"cron\") returned ok=false") }
	if s.GroupID != "advanced" {
		t.Errorf("GroupID = %q, want advanced", s.GroupID)
	}
	if s.Shape != ShapeList {
		t.Errorf("Shape = %v, want ShapeList", s.Shape)
	}
	if s.Subkey != "jobs" {
		t.Errorf("Subkey = %q, want \"jobs\"", s.Subkey)
	}
	if !s.NoDiscriminator {
		t.Error("NoDiscriminator = false, want true")
	}
}

func TestCronFields(t *testing.T) {
	s, _ := Get("cron")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields { byName[s.Fields[i].Name] = &s.Fields[i] }
	for _, want := range []struct{ name string; kind FieldKind; required bool }{
		{"name", FieldString, true},
		{"schedule", FieldString, true},
		{"prompt", FieldString, true},
		{"model", FieldString, false},
	} {
		f, ok := byName[want.name]
		if !ok { t.Errorf("field %q missing", want.name); continue }
		if f.Kind != want.kind {
			t.Errorf("%s.Kind = %s, want %s", want.name, f.Kind, want.kind)
		}
		if f.Required != want.required {
			t.Errorf("%s.Required = %v, want %v", want.name, f.Required, want.required)
		}
	}
	// No "provider" field should exist.
	if _, has := byName["provider"]; has {
		t.Error("cron should not have a provider field (NoDiscriminator mode)")
	}
}
```

- [ ] **Step 2: Run — expect failure**

Run: `cd config/descriptor && go test -run TestCron`

- [ ] **Step 3: Create `cron.go`**

```go
package descriptor

// Cron mirrors config.CronConfig. YAML layout: cron.jobs: [...].
// Each job is {name, schedule, prompt, model} — uniform, no discriminator.
// Subkey="jobs" tells the API redact/preserve pipeline and the UI to
// unwrap one extra YAML layer; NoDiscriminator opts out of the
// "exactly one FieldEnum named provider" ShapeList invariant.
func init() {
	Register(Section{
		Key:     "cron",
		Label:   "Cron jobs",
		Summary: "Scheduled prompts that run on a recurring schedule.",
		GroupID: "advanced",
		Shape:   ShapeList,
		Subkey:  "jobs",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "name", Label: "Name", Kind: FieldString, Required: true,
				Help: "Stable identifier for this job."},
			{Name: "schedule", Label: "Schedule", Kind: FieldString, Required: true,
				Help: `Cron expression or "every 5m" / "every 1h" shorthand.`},
			{Name: "prompt", Label: "Prompt", Kind: FieldString, Required: true,
				Help: "Prompt text to send. For long prompts, edit YAML directly."},
			{Name: "model", Label: "Model override", Kind: FieldString,
				Help: "Leave blank to use the default model. Falls back to config.model.",
				DatalistSource: &DatalistSource{Section: "providers", Field: "model"}},
		},
	})
}
```

- [ ] **Step 4: Run — expect pass**

Run: `cd config/descriptor && go test -run TestCron`
Also: `cd config/descriptor && go test ./...`

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/cron.go config/descriptor/cron_test.go
git commit -m "feat(config/descriptor): Cron section — ShapeList with Subkey=jobs, NoDiscriminator for uniform jobs"
```

---

### Task 9: Register `mcp` descriptor (ShapeKeyedMap, Subkey, NoDiscriminator)

**Files:**
- Create: `config/descriptor/mcp.go`
- Create: `config/descriptor/mcp_test.go`

**Why:** MCP is ShapeKeyedMap with Subkey="servers" and NoDiscriminator (each server is keyed by its name, no type discriminator). Args ([]string) and Env (map[string]string) are deferred — UI ships with `command` + `enabled` only.

- [ ] **Step 1: Write failing tests**

Create `config/descriptor/mcp_test.go`:

```go
package descriptor

import "testing"

func TestMCPSectionRegistered(t *testing.T) {
	s, ok := Get("mcp")
	if !ok { t.Fatal("Get(\"mcp\") returned ok=false") }
	if s.GroupID != "advanced" {
		t.Errorf("GroupID = %q, want advanced", s.GroupID)
	}
	if s.Shape != ShapeKeyedMap {
		t.Errorf("Shape = %v, want ShapeKeyedMap", s.Shape)
	}
	if s.Subkey != "servers" {
		t.Errorf("Subkey = %q, want \"servers\"", s.Subkey)
	}
	if !s.NoDiscriminator {
		t.Error("NoDiscriminator = false, want true")
	}
}

func TestMCPFields(t *testing.T) {
	s, _ := Get("mcp")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields { byName[s.Fields[i].Name] = &s.Fields[i] }
	if byName["command"] == nil || byName["command"].Kind != FieldString ||
		!byName["command"].Required {
		t.Error("command field missing or wrong: want FieldString + Required")
	}
	if byName["enabled"] == nil || byName["enabled"].Kind != FieldBool {
		t.Error("enabled field missing or wrong: want FieldBool")
	}
	if _, has := byName["provider"]; has {
		t.Error("mcp should not have a provider field (NoDiscriminator mode)")
	}
}
```

- [ ] **Step 2: Run — expect failure**

Run: `cd config/descriptor && go test -run TestMCP`

- [ ] **Step 3: Create `mcp.go`**

```go
package descriptor

// MCP mirrors config.MCPConfig. YAML layout: mcp.servers.<name>.{command,...}.
// Each server keyed by its name (ShapeKeyedMap) with uniform fields
// (NoDiscriminator — no provider/type field; instance name IS the identity).
//
// Args ([]string) and Env (map[string]string) are NOT exposed in the UI
// yet — those need new FieldStringList / FieldStringMap kinds. Users
// with complex MCP setups should edit config.yaml directly for now; the
// UI still lets them toggle Enabled and rename Command.
func init() {
	Register(Section{
		Key:     "mcp",
		Label:   "MCP servers",
		Summary: "Model Context Protocol servers launched on CLI/gateway startup.",
		GroupID: "advanced",
		Shape:   ShapeKeyedMap,
		Subkey:  "servers",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "command", Label: "Command", Kind: FieldString, Required: true,
				Help: "Executable to run, e.g. \"npx\" or a full path."},
			{Name: "enabled", Label: "Enabled", Kind: FieldBool,
				Help: "Disabled servers never start. Delete the entry to remove it entirely. Edit config.yaml directly to configure args/env."},
		},
	})
}
```

- [ ] **Step 4: Run — expect pass**

Run: `cd config/descriptor && go test -run TestMCP`
Also: `cd config/descriptor && go test ./...`

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/mcp.go config/descriptor/mcp_test.go
git commit -m "feat(config/descriptor): MCP section — ShapeKeyedMap with Subkey=servers (command+enabled; args/env deferred)"
```

---

### Task 10: Flip `advanced` group to `done`

**Files:**
- Modify: `web/src/shell/groups.ts`

**Why:** The shell's sidebar uses `plannedStage` to decide between descriptor-backed rendering and the `ComingSoonPanel` fallback. Three descriptors are now registered for the group, so flip it to `'done'`.

- [ ] **Step 1: Edit groups.ts**

```ts
  {
    id: 'advanced',
    label: 'Advanced',
    plannedStage: 'done',  // was: '7'
    configKeys: ['mcp', 'browser', 'cron'],
    ...
  },
```

- [ ] **Step 2: Run the shell tests**

Run: `cd web && npm test -- shell`
Expected: all pass. If a test asserts `plannedStage: '7'`, update the assertion.

- [ ] **Step 3: Commit**

```bash
git add web/src/shell/groups.ts
git commit -m "feat(web/shell): flip Advanced group plannedStage 7 -> done"
```

---

### Task 11: Web bundle rebuild + smoke doc

**Files:**
- Rebuild: `api/webroot/assets/*.js` (via web build step)
- Create: `docs/smoke/advanced.md`

**Why:** The Go binary embeds `api/webroot` via `//go:embed`. Any TS change requires rebuilding. And since this plan adds meaningful surface area, a smoke doc captures how to verify the flow manually.

- [ ] **Step 1: Rebuild the web bundle**

Run: `cd web && npm run build` (or whatever the repo's build command is — check web/package.json scripts).
Verify: `api/webroot/index.html` references a newly-hashed `assets/index-*.js`.
The old bundle file (e.g. `index-<oldhash>.js`) should be removed by the build; if it lingers, delete it manually. Confirm only one `assets/index-*.js` exists.

- [ ] **Step 2: Write the smoke doc**

Create `docs/smoke/advanced.md`:

```markdown
# Advanced Group Smoke Flow

Verify the `browser`, `mcp`, and `cron` config editors.

## Setup

```bash
# Start a clean config dir
export HERMIND_HOME=/tmp/hermind-advanced-smoke
mkdir -p "$HERMIND_HOME"
cat > "$HERMIND_HOME/config.yaml" <<'EOF'
browser:
  provider: browserbase
  browserbase:
    api_key: secret-test-key
    project_id: proj-123
mcp:
  servers:
    smoketest:
      command: /bin/echo
      enabled: true
cron:
  jobs:
    - name: hourly-ping
      schedule: every 1h
      prompt: What time is it?
EOF

# Launch the server
go run ./cmd/hermind &
```

## Browser

1. Open http://127.0.0.1:9119/#advanced.
2. Click **Browser**. Expect a `provider` dropdown with three options.
3. Select `browserbase` — the five browserbase fields render. `api_key` shows the redacted placeholder, not the real value.
4. Edit `base_url` → hit Save. Reload. Browserbase fields still render; `api_key` is still redacted.
5. Switch to `camofox` — `base_url` + `managed_persistence` render instead.
6. Switch to blank — all sub-fields hide.

## MCP

1. Click **MCP servers**. See instance picker with `smoketest`.
2. Select `smoketest` — see `command` = `/bin/echo`, `enabled` = true.
3. Click **Add server** — dialog should ask only for a name (no provider dropdown). Type `probe` and confirm.
4. Fill `command = /bin/true`, toggle Enabled, Save.
5. Reload — both servers still there.

## Cron

1. Click **Cron jobs**. See list with `hourly-ping`.
2. Click the job — see name, schedule, prompt, model fields.
3. Click **Add job** — appends a blank element, no provider picker.
4. Fill fields, save, reload.

## Cleanup

```bash
kill %1
rm -rf /tmp/hermind-advanced-smoke
```
```

- [ ] **Step 3: Commit**

```bash
git add api/webroot docs/smoke/advanced.md
git commit -m "chore(web): rebuild bundle + smoke doc for Advanced group"
```

---

## Self-Review Checklist

- **Spec coverage** — three descriptors registered (browser/mcp/cron), framework extended with Subkey + NoDiscriminator, API redact/preserve + schema DTO updated, web UI routed + new-instance UX adapted, group flipped to `done`, smoke doc written. ✅
- **Placeholder scan** — all code blocks show actual code; "adapt to existing helper" comments only appear next to concrete instructions to grep a specific file for a name. No TBDs.
- **Type consistency** — `Subkey string` and `NoDiscriminator bool` are introduced in Task 1 and referenced with the same names in Tasks 2–10. JSON keys are `subkey` / `no_discriminator` throughout.
- **Known unfixable gaps** — Task 5 / Task 6 reference props (`onFieldChange`, `activeSubKey`) whose exact signatures depend on the current ContentPanel / App.tsx implementations. The plan flags this explicitly and tells implementers to read the relevant ranges before editing. That is acceptable because a precise mechanical spec would duplicate the current code in the plan, and rot the moment anyone touched App.tsx.

## Deferred Follow-ups (not in this plan)

- `FieldStringList` + `FieldStringMap` kinds → re-open mcp to add `args` and `env`.
- `FieldTextArea` → cron `prompt` long-text editing.
- Env-override badges → show a "env overrides this" indicator on browserbase fields when BROWSERBASE_API_KEY / BROWSERBASE_PROJECT_ID are set in the server environment.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-21-advanced-group-descriptors.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, spec-compliance + code-quality review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using `superpowers:executing-plans`; batch with checkpoints.

**Which approach?**
