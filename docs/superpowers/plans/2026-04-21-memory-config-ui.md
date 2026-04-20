# Memory Config UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fill the empty "Memory" group in the web config UI with a full form for `config.MemoryConfig`'s 8 external-memory backends (Honcho, Mem0, Supermemory, Hindsight, RetainDB, OpenViking, Byterover, Holographic).

**Architecture:** Ship reusable dotted-path infrastructure (~40 lines across `path.ts` / `ConfigSection.tsx` / `state.ts` / `handlers_config.go`) first, then register one Memory descriptor (~95 lines) with `provider` as a `FieldEnum` discriminator and all per-backend fields gated by `VisibleWhen`. Non-dotted field names keep working identically — every existing section is untouched.

**Tech Stack:** Go 1.22+ (backend), React 18 + TypeScript + Vitest (frontend), descriptor-based schema system in `config/descriptor/`.

**Spec:** `docs/superpowers/specs/2026-04-21-memory-config-ui-design.md`

---

## File Structure

**Created:**
- `web/src/util/path.ts` — `getPath(obj, path)` / `setPath(obj, path, value)` pure helpers.
- `web/src/util/path.test.ts` — 6 unit tests for the helpers.
- `config/descriptor/memory.go` — Memory section registration.
- `config/descriptor/memory_test.go` — schema pin tests.
- `docs/smoke/memory.md` — operator verification flow.

**Modified:**
- `web/src/components/ConfigSection.tsx` — read/write and `isVisible` use `getPath`.
- `web/src/components/ConfigSection.test.tsx` — add one dotted-path case.
- `web/src/state.ts` — `edit/config-field` reducer uses `setPath`.
- `web/src/state.test.ts` — add one dotted-path case.
- `api/handlers_config.go` — `redactSectionSecrets` and `preserveSectionSecrets` walk dotted paths in their `ShapeMap` branches via a new `walkPath` helper.
- `api/handlers_config_test.go` — add two dotted-path cases.

**Untouched (will pick up the new Memory section automatically):**
- `config/config.go` — `MemoryConfig` struct is unchanged.
- `web/src/components/shell/Sidebar.tsx`, `SectionList.tsx` — already filter `configSections` by `group_id === 'memory'`; registering the descriptor makes the rows appear.
- Every other descriptor file (`storage.go`, `auxiliary.go`, etc.) — flat names keep working through the same helpers.

---

### Task 1: Dotted-path utility

**Files:**
- Create: `web/src/util/path.ts`
- Create: `web/src/util/path.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `web/src/util/path.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { getPath, setPath } from './path';

describe('getPath', () => {
  it('returns a flat field', () => {
    expect(getPath({ a: 1 }, 'a')).toBe(1);
  });
  it('returns a nested field', () => {
    expect(getPath({ a: { b: 2 } }, 'a.b')).toBe(2);
  });
  it('returns undefined when an intermediate key is missing', () => {
    expect(getPath({}, 'a.b')).toBeUndefined();
  });
});

describe('setPath', () => {
  it('writes a flat field', () => {
    expect(setPath({ a: 1 }, 'a', 2)).toEqual({ a: 2 });
  });
  it('writes a nested field, creating intermediates', () => {
    expect(setPath({}, 'a.b', 2)).toEqual({ a: { b: 2 } });
  });
  it('does not mutate the input', () => {
    const input = { a: { b: 1 } };
    const out = setPath(input, 'a.b', 2);
    expect(input).toEqual({ a: { b: 1 } });
    expect(out).toEqual({ a: { b: 2 } });
    expect(out).not.toBe(input);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && bunx vitest run src/util/path.test.ts`
Expected: FAIL — module `./path` not found.

- [ ] **Step 3: Implement the helpers**

Create `web/src/util/path.ts`:

```ts
// getPath walks a dotted path down an object and returns the leaf value.
// Returns undefined if any intermediate key is missing or the root is not
// an object. Flat (no-dot) paths behave exactly like obj[path].
export function getPath(obj: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>(
    (o, k) => (o as Record<string, unknown> | undefined)?.[k],
    obj,
  );
}

// setPath returns a new object with value written at path. Intermediate
// objects are created as empty {} when missing. The input is never
// mutated; flat paths produce {...obj, [head]: value}.
export function setPath(
  obj: Record<string, unknown>,
  path: string,
  value: unknown,
): Record<string, unknown> {
  const [head, ...rest] = path.split('.');
  if (rest.length === 0) return { ...obj, [head]: value };
  const inner = (obj[head] as Record<string, unknown> | undefined) ?? {};
  return { ...obj, [head]: setPath(inner, rest.join('.'), value) };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bunx vitest run src/util/path.test.ts`
Expected: PASS, 6/6 tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/util/path.ts web/src/util/path.test.ts
git commit -m "feat(web/util): getPath / setPath — immutable dotted-path helpers"
```

---

### Task 2: ConfigSection renders dotted field names

**Files:**
- Modify: `web/src/components/ConfigSection.tsx:59-62`, `web/src/components/ConfigSection.tsx:105-111`
- Modify: `web/src/components/ConfigSection.test.tsx` — append new test

**Context:** `ConfigSection.tsx` currently uses `value[f.name]` (flat index) in three places: the current value at `:59`, the original value at `:60`, and the `visible_when` check in `isVisible` at `:110`. All three need to switch to `getPath`. The `onChange` wiring at `:62` stays unchanged — it passes the dotted name straight to the parent. Read `web/src/components/ConfigSection.tsx` before editing.

- [ ] **Step 1: Write the failing test**

Append to `web/src/components/ConfigSection.test.tsx`:

```tsx
const nested: ConfigSectionT = {
  key: 'memory',
  label: 'Memory',
  group_id: 'memory',
  fields: [
    { name: 'provider', label: 'Provider', kind: 'enum', enum: ['', 'honcho'] },
    {
      name: 'honcho.api_key', label: 'Honcho API key', kind: 'secret',
      visible_when: { field: 'provider', equals: 'honcho' },
    },
  ],
};

describe('ConfigSection — dotted field names', () => {
  it('renders a nested value and dispatches the dotted name on edit', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();

    function Host() {
      const [value, setValue] = useState<Record<string, unknown>>({
        provider: 'honcho',
        honcho: { api_key: 'orig-key' },
      });
      return (
        <ConfigSection
          section={nested}
          value={value}
          originalValue={value}
          onFieldChange={(name, v) => {
            setValue(prev => {
              // Host mirrors the reducer behavior for the test.
              if (!name.includes('.')) return { ...prev, [name]: v };
              const [head, ...rest] = name.split('.');
              const inner = (prev[head] as Record<string, unknown>) ?? {};
              return { ...prev, [head]: { ...inner, [rest.join('.')]: v } };
            });
            onFieldChange(name, v);
          }}
        />
      );
    }

    render(<Host />);
    const input = screen.getByLabelText(/honcho api key/i) as HTMLInputElement;
    expect(input.value).toBe('orig-key');
    await user.clear(input);
    await user.type(input, 'new-key');
    const last = onFieldChange.mock.calls[onFieldChange.mock.calls.length - 1];
    expect(last[0]).toBe('honcho.api_key');
    expect(last[1]).toBe('new-key');
  });

  it('hides a dotted-name field when the sibling discriminator does not match', () => {
    render(
      <ConfigSection
        section={nested}
        value={{ provider: '' }}
        originalValue={{ provider: '' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText(/honcho api key/i)).toBeNull();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && bunx vitest run src/components/ConfigSection.test.tsx`
Expected: the two new tests FAIL — `value[f.name]` returns `undefined` for `"honcho.api_key"`, so the SecretInput renders empty and the visible_when predicate returns `false`.

- [ ] **Step 3: Edit `web/src/components/ConfigSection.tsx`**

Add the import at the top of the file (after the existing imports):

```tsx
import { getPath } from '../util/path';
```

Replace lines `59-60`:

```tsx
const current = asString(value[f.name]);
const original = asString(originalValue[f.name]);
```

with:

```tsx
const current = asString(getPath(value, f.name));
const original = asString(getPath(originalValue, f.name));
```

Replace the body of `isVisible` (lines `105-111`):

```tsx
function isVisible(f: ConfigField, value: Record<string, unknown>): boolean {
  if (!f.visible_when) return true;
  return String(value[f.visible_when.field]) === String(f.visible_when.equals);
}
```

with:

```tsx
function isVisible(f: ConfigField, value: Record<string, unknown>): boolean {
  if (!f.visible_when) return true;
  // Values arrive as real types on boot (bool true, number 42) but edited
  // values pass through string-coerced field onChange handlers. Coerce both
  // sides to string so predicates keep matching either way. getPath handles
  // both flat and dotted discriminator names (e.g. "provider" or "foo.bar").
  return String(getPath(value, f.visible_when.field)) === String(f.visible_when.equals);
}
```

- [ ] **Step 4: Run tests to verify all pass**

Run: `cd web && bunx vitest run src/components/ConfigSection.test.tsx`
Expected: PASS, all tests (original + 2 new).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ConfigSection.tsx web/src/components/ConfigSection.test.tsx
git commit -m "feat(web): ConfigSection resolves dotted field names via getPath"
```

---

### Task 3: state reducer writes dotted field names

**Files:**
- Modify: `web/src/state.ts:161-171`
- Modify: `web/src/state.test.ts` — append new test in `describe('edit/config-field', …)`

**Context:** The `edit/config-field` reducer currently writes `{ ...prev, [action.field]: action.value }` at `state.ts:168`. For a dotted name like `honcho.api_key` this puts the value at a literal `"honcho.api_key"` key, clobbering nothing useful. We swap the spread for `setPath` from Task 1 — flat names still work identically; dotted names walk down. Read `web/src/state.ts` around line 161 before editing.

- [ ] **Step 1: Write the failing test**

Append inside the existing `describe('edit/config-field', …)` block in `web/src/state.test.ts`:

```ts
it('writes a dotted field name into a nested object without clobbering siblings', () => {
  const state: AppState = reducer(initialState, {
    type: 'boot/loaded',
    descriptors: emptyDescriptors,
    configSections: [],
    config: { memory: { provider: 'honcho', honcho: { workspace: 'w' } } } as unknown as Config,
  });
  const next = reducer(state, {
    type: 'edit/config-field',
    sectionKey: 'memory',
    field: 'honcho.api_key',
    value: 'k',
  });
  const cfg = next.config as unknown as Record<string, unknown>;
  const memory = cfg.memory as Record<string, unknown>;
  const honcho = memory.honcho as Record<string, unknown>;
  expect(honcho.workspace).toBe('w');
  expect(honcho.api_key).toBe('k');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && bunx vitest run src/state.test.ts -t "dotted field name"`
Expected: FAIL — reducer writes to a literal `"honcho.api_key"` key on `memory`, so `memory.honcho.api_key` is undefined.

- [ ] **Step 3: Edit the reducer**

In `web/src/state.ts`, add the import after the existing imports:

```ts
import { setPath } from './util/path';
```

Replace the existing `edit/config-field` case body (lines `161-171`):

```ts
case 'edit/config-field': {
  const cfg = state.config as unknown as Record<string, unknown>;
  const prev = (cfg[action.sectionKey] as Record<string, unknown> | undefined) ?? {};
  return {
    ...state,
    config: {
      ...state.config,
      [action.sectionKey]: { ...prev, [action.field]: action.value },
    } as typeof state.config,
  };
}
```

with:

```ts
case 'edit/config-field': {
  const cfg = state.config as unknown as Record<string, unknown>;
  const prev = (cfg[action.sectionKey] as Record<string, unknown> | undefined) ?? {};
  return {
    ...state,
    config: {
      ...state.config,
      [action.sectionKey]: setPath(prev, action.field, action.value),
    } as typeof state.config,
  };
}
```

- [ ] **Step 4: Run all state tests to verify they pass**

Run: `cd web && bunx vitest run src/state.test.ts`
Expected: PASS — existing flat-name tests still pass (setPath's no-dot branch produces `{...obj, [head]: value}`, identical to the prior spread), plus the new dotted-name case.

- [ ] **Step 5: Commit**

```bash
git add web/src/state.ts web/src/state.test.ts
git commit -m "feat(web/state): edit/config-field resolves dotted field names via setPath"
```

---

### Task 4: Backend secret helpers walk dotted paths

**Files:**
- Modify: `api/handlers_config.go:77-140` (`redactSectionSecrets`), `:211-352` (`preserveSectionSecrets`)
- Modify: `api/handlers_config_test.go` — append two new tests

**Context:** `redactSectionSecrets` and `preserveSectionSecrets` handle secrets in every descriptor shape. The `ShapeMap` branch (both functions' final block) indexes by `blob[f.Name]` / `upd[f.Name]` / `cur[f.Name]`. For a dotted field like `honcho.api_key`, this misses the actual leaf. We add a shared `walkPath` helper that returns `(parent, leafKey, found)`, and rewrite just the `ShapeMap` branches. `ShapeKeyedMap` / `ShapeList` / `ShapeScalar` branches are unchanged.

We also need to register a test-only Memory-like descriptor in the test file so the helper has a `FieldSecret` with a dotted name to act on — **but** we don't want the test to depend on the real `memory` descriptor's shape (Task 5 ships that). The tests use ad-hoc descriptors registered and deregistered around the test body.

The existing test file uses `RedactSectionSecretsForTest` and `PreserveSectionSecretsForTest` — we keep using these.

Read `api/handlers_config.go` around `redactSectionSecrets` and `preserveSectionSecrets` before editing.

- [ ] **Step 1: Write the failing tests**

Append to `api/handlers_config_test.go`:

```go
func TestRedactSectionSecrets_DottedPath(t *testing.T) {
	const key = "dotted_redact_test"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Dotted Redact",
		GroupID: "runtime",
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Provider", Kind: descriptor.FieldEnum,
				Enum: []string{"", "a"}},
			{Name: "a.api_key", Label: "A API key", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	blob := map[string]any{
		key: map[string]any{
			"provider": "a",
			"a": map[string]any{
				"api_key": "sk-dotted-secret",
			},
		},
	}
	RedactSectionSecretsForTest(blob)
	outer, _ := blob[key].(map[string]any)
	inner, _ := outer["a"].(map[string]any)
	if inner["api_key"] != "" {
		t.Errorf("%s.a.api_key = %q, want \"\" (redacted)", key, inner["api_key"])
	}
}

func TestPreserveSectionSecrets_DottedPath(t *testing.T) {
	const key = "dotted_preserve_test"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Dotted Preserve",
		GroupID: "runtime",
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Provider", Kind: descriptor.FieldEnum,
				Enum: []string{"", "a"}},
			{Name: "a.api_key", Label: "A API key", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

	current := map[string]any{
		key: map[string]any{
			"provider": "a",
			"a":        map[string]any{"api_key": "sk-real"},
		},
	}
	updated := map[string]any{
		key: map[string]any{
			"provider": "a",
			"a":        map[string]any{"api_key": ""}, // blanked by redact
		},
	}
	PreserveSectionSecretsForTest(updated, current)
	outer, _ := updated[key].(map[string]any)
	inner, _ := outer["a"].(map[string]any)
	if inner["api_key"] != "sk-real" {
		t.Errorf("%s.a.api_key = %q, want %q (restored)", key, inner["api_key"], "sk-real")
	}
}
```

- [ ] **Step 2: Add `descriptor.Unregister` (needed by test cleanup)**

The test cleanup calls `descriptor.Unregister(key)`. That function doesn't exist yet. Read `config/descriptor/descriptor.go` around `func Register` (line `116-122`).

Append to `config/descriptor/descriptor.go`:

```go
// Unregister removes the section at key. It's intended for tests that
// register an ad-hoc descriptor and need to tear it down afterward so
// subsequent tests see a clean registry.
func Unregister(key string) {
	delete(registry, key)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./api -run "TestRedactSectionSecrets_DottedPath|TestPreserveSectionSecrets_DottedPath" -v`
Expected: FAIL — redact leaves `api_key` non-empty, preserve doesn't restore.

- [ ] **Step 4: Add the `walkPath` helper in `api/handlers_config.go`**

Add the `strings` import if not already present (it is — `handlers_config.go` doesn't use it yet, so add it). Append after the existing imports:

```go
import "strings"
```

(if `strings` is already imported, skip this step.)

Append this helper at the end of `api/handlers_config.go`:

```go
// walkPath follows dotted keys down m, returning the leaf's parent map and
// the final key. ok=false means the path is missing at some intermediate
// level (not the last key); callers skip that field. Flat paths (no dot)
// return (m, path, true) so callers need no special case.
func walkPath(m map[string]any, path string) (parent map[string]any, leaf string, ok bool) {
	keys := strings.Split(path, ".")
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			return cur, k, true
		}
		next, isMap := cur[k].(map[string]any)
		if !isMap {
			return nil, "", false
		}
		cur = next
	}
	return nil, "", false // unreachable when keys is non-empty
}
```

- [ ] **Step 5: Rewrite the `ShapeMap` branch of `redactSectionSecrets`**

In `api/handlers_config.go`, find the final block of `redactSectionSecrets` (roughly `:127-139`):

```go
blob, ok := m[sec.Key].(map[string]any)
if !ok {
    continue
}
for _, f := range sec.Fields {
    if f.Kind != descriptor.FieldSecret {
        continue
    }
    if _, present := blob[f.Name]; present {
        blob[f.Name] = ""
    }
}
```

Replace with:

```go
blob, ok := m[sec.Key].(map[string]any)
if !ok {
    continue
}
for _, f := range sec.Fields {
    if f.Kind != descriptor.FieldSecret {
        continue
    }
    parent, leaf, found := walkPath(blob, f.Name)
    if !found {
        continue
    }
    if _, present := parent[leaf]; present {
        parent[leaf] = ""
    }
}
```

- [ ] **Step 6: Rewrite the `ShapeMap` branch of `preserveSectionSecrets`**

Find the final block of `preserveSectionSecrets` (roughly `:316-342`):

```go
upd, ok := updM[sec.Key].(map[string]any)
if !ok {
    continue
}
cur, _ := curM[sec.Key].(map[string]any)
for _, f := range sec.Fields {
    if f.Kind != descriptor.FieldSecret {
        continue
    }
    newVal, _ := upd[f.Name].(string)
    if newVal != "" {
        continue
    }
    if cur == nil {
        continue
    }
    prevVal, _ := cur[f.Name].(string)
    if prevVal == "" {
        continue
    }
    upd[f.Name] = prevVal
    changed = true
}
if changed {
    updM[sec.Key] = upd
}
```

Replace with:

```go
upd, ok := updM[sec.Key].(map[string]any)
if !ok {
    continue
}
cur, _ := curM[sec.Key].(map[string]any)
for _, f := range sec.Fields {
    if f.Kind != descriptor.FieldSecret {
        continue
    }
    updParent, leaf, updFound := walkPath(upd, f.Name)
    if !updFound {
        continue
    }
    newVal, _ := updParent[leaf].(string)
    if newVal != "" {
        continue
    }
    if cur == nil {
        continue
    }
    curParent, _, curFound := walkPath(cur, f.Name)
    if !curFound {
        continue
    }
    prevVal, _ := curParent[leaf].(string)
    if prevVal == "" {
        continue
    }
    updParent[leaf] = prevVal
    changed = true
}
if changed {
    updM[sec.Key] = upd
}
```

- [ ] **Step 7: Also update `PreserveSectionSecretsForTest` for parity**

The test-only wrapper in `api/handlers_config.go:364-434` mimics the production loop. Since Task 4's tests use it, the `ShapeMap` branch needs the same dotted-path handling. But — looking at the wrapper — it currently has no `ShapeMap` branch at all (it only iterates `ShapeKeyedMap` and `ShapeList`).

Add the `ShapeMap` branch to `PreserveSectionSecretsForTest` at the end of the outer loop body (right before the closing `}` of the `for _, sec := range descriptor.All()` block):

```go
// ShapeMap branch — mirrors production preserveSectionSecrets' final block.
upd, ok := updated[sec.Key].(map[string]any)
if !ok {
    continue
}
cur, _ := current[sec.Key].(map[string]any)
for _, f := range sec.Fields {
    if f.Kind != descriptor.FieldSecret {
        continue
    }
    updParent, leaf, updFound := walkPath(upd, f.Name)
    if !updFound {
        continue
    }
    newVal, _ := updParent[leaf].(string)
    if newVal != "" {
        continue
    }
    if cur == nil {
        continue
    }
    curParent, _, curFound := walkPath(cur, f.Name)
    if !curFound {
        continue
    }
    prevVal, _ := curParent[leaf].(string)
    if prevVal == "" {
        continue
    }
    updParent[leaf] = prevVal
}
```

- [ ] **Step 8: Run dotted-path tests to verify they pass**

Run: `go test ./api -run "TestRedactSectionSecrets_DottedPath|TestPreserveSectionSecrets_DottedPath" -v`
Expected: PASS — both cases.

- [ ] **Step 9: Run full backend test suite to catch regressions**

Run: `go test ./...`
Expected: PASS — no test changes its behavior because every existing descriptor uses flat names, and `walkPath("field")` returns `(m, "field", true)` — identical to the pre-change indexing.

- [ ] **Step 10: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go config/descriptor/descriptor.go
git commit -m "feat(api): walkPath helper — redact/preserve handle dotted field names"
```

---

### Task 5: Memory descriptor registration

**Files:**
- Create: `config/descriptor/memory.go`
- Create: `config/descriptor/memory_test.go`

**Context:** With Tasks 1-4 landed, registering a section with dotted field names is all that's needed for the Memory form to render in the web UI. `config.MemoryConfig` in `config/config.go:105-115` has a `Provider` enum plus 8 nested sub-structs — `Honcho`, `Mem0`, `Supermemory`, `Hindsight`, `RetainDB`, `OpenViking`, `Byterover`, `Holographic`. Holographic has zero fields (placeholder); Byterover has no `api_key` (CLI wrapper); every other backend has `base_url` + `api_key` + some identifier field(s). Hindsight has a `budget` enum (`low/mid/high`).

- [ ] **Step 1: Write the failing tests**

Create `config/descriptor/memory_test.go`:

```go
package descriptor

import "testing"

func TestMemorySectionRegistered(t *testing.T) {
	s, ok := Get("memory")
	if !ok {
		t.Fatalf("Get(\"memory\") returned ok=false — did memory.go init() register?")
	}
	if s.GroupID != "memory" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "memory")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestMemoryProviderIsEnumWithAllBackends(t *testing.T) {
	s, _ := Get("memory")
	var provider *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "provider" {
			provider = &s.Fields[i]
			break
		}
	}
	if provider == nil {
		t.Fatal("provider field not found")
	}
	if provider.Kind != FieldEnum {
		t.Errorf("provider.Kind = %s, want enum", provider.Kind)
	}
	want := map[string]bool{
		"": true, "honcho": true, "mem0": true, "supermemory": true,
		"hindsight": true, "retaindb": true, "openviking": true,
		"byterover": true, "holographic": true,
	}
	got := map[string]bool{}
	for _, v := range provider.Enum {
		got[v] = true
	}
	for v := range want {
		if !got[v] {
			t.Errorf("provider.Enum missing %q, got %v", v, provider.Enum)
		}
	}
}

func TestMemoryAPIKeysAreSecretAndGatedByProvider(t *testing.T) {
	s, _ := Get("memory")
	wantSecrets := []string{
		"honcho.api_key", "mem0.api_key", "supermemory.api_key",
		"hindsight.api_key", "retaindb.api_key", "openviking.api_key",
	}
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range wantSecrets {
		f, ok := byName[name]
		if !ok {
			t.Errorf("field %q not found", name)
			continue
		}
		if f.Kind != FieldSecret {
			t.Errorf("field %q: Kind = %s, want secret", name, f.Kind)
		}
		if f.VisibleWhen == nil {
			t.Errorf("field %q: VisibleWhen is nil", name)
			continue
		}
		if f.VisibleWhen.Field != "provider" {
			t.Errorf("field %q: VisibleWhen.Field = %q, want \"provider\"",
				name, f.VisibleWhen.Field)
		}
		// Backend name is the first dotted segment — equals check target.
		// e.g. "honcho.api_key" -> want Equals == "honcho".
		var backend string
		for i := 0; i < len(name); i++ {
			if name[i] == '.' {
				backend = name[:i]
				break
			}
		}
		if f.VisibleWhen.Equals != backend {
			t.Errorf("field %q: VisibleWhen.Equals = %v, want %q",
				name, f.VisibleWhen.Equals, backend)
		}
	}
}

func TestMemoryByteroverFieldsAreGated(t *testing.T) {
	s, _ := Get("memory")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range []string{"byterover.brv_path", "byterover.cwd"} {
		f, ok := byName[name]
		if !ok {
			t.Errorf("field %q not found", name)
			continue
		}
		if f.Kind != FieldString {
			t.Errorf("field %q: Kind = %s, want string", name, f.Kind)
		}
		if f.VisibleWhen == nil || f.VisibleWhen.Field != "provider" ||
			f.VisibleWhen.Equals != "byterover" {
			t.Errorf("field %q: VisibleWhen = %+v, want {provider=byterover}",
				name, f.VisibleWhen)
		}
	}
}

func TestMemoryHindsightBudgetIsEnum(t *testing.T) {
	s, _ := Get("memory")
	var budget *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "hindsight.budget" {
			budget = &s.Fields[i]
			break
		}
	}
	if budget == nil {
		t.Fatal("hindsight.budget not found")
	}
	if budget.Kind != FieldEnum {
		t.Errorf("hindsight.budget.Kind = %s, want enum", budget.Kind)
	}
	want := map[string]bool{"low": true, "mid": true, "high": true}
	for _, v := range budget.Enum {
		delete(want, v)
	}
	if len(want) > 0 {
		t.Errorf("hindsight.budget.Enum missing %v, got %v", want, budget.Enum)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./config/descriptor -run "TestMemory" -v`
Expected: FAIL — all 5 tests fail because no `memory.go` exists yet.

- [ ] **Step 3: Implement the Memory descriptor**

Create `config/descriptor/memory.go`:

```go
package descriptor

// Memory mirrors config.MemoryConfig. The provider field is a FieldEnum
// discriminator: when blank, no external memory is configured (matches
// the yaml "omitempty" semantics on MemoryConfig.Provider). Each non-
// Holographic backend has sub-fields gated by VisibleWhen so only the
// active backend's inputs render.
//
// Dotted field names like "honcho.api_key" require the dotted-path
// infrastructure in ConfigSection.tsx, state.ts (edit/config-field
// reducer), and api/handlers_config.go (walkPath helper).
func init() {
	gate := func(backend string) *Predicate {
		return &Predicate{Field: "provider", Equals: backend}
	}
	Register(Section{
		Key:     "memory",
		Label:   "Memory",
		Summary: "Optional external long-term memory provider. Leave blank for no external memory.",
		GroupID: "memory",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "provider",
				Label: "Provider",
				Help:  "External memory backend. Leave blank for no external memory.",
				Kind:  FieldEnum,
				Enum: []string{
					"", "honcho", "mem0", "supermemory", "hindsight",
					"retaindb", "openviking", "byterover", "holographic",
				},
			},

			// Honcho
			{Name: "honcho.base_url", Label: "Honcho base URL",
				Kind: FieldString, VisibleWhen: gate("honcho")},
			{Name: "honcho.api_key", Label: "Honcho API key",
				Kind: FieldSecret, VisibleWhen: gate("honcho")},
			{Name: "honcho.workspace", Label: "Honcho workspace",
				Kind: FieldString, VisibleWhen: gate("honcho")},
			{Name: "honcho.peer", Label: "Honcho peer",
				Kind: FieldString, VisibleWhen: gate("honcho")},

			// Mem0
			{Name: "mem0.base_url", Label: "Mem0 base URL",
				Kind: FieldString, VisibleWhen: gate("mem0")},
			{Name: "mem0.api_key", Label: "Mem0 API key",
				Kind: FieldSecret, VisibleWhen: gate("mem0")},
			{Name: "mem0.user_id", Label: "Mem0 user ID",
				Kind: FieldString, VisibleWhen: gate("mem0")},

			// Supermemory
			{Name: "supermemory.base_url", Label: "Supermemory base URL",
				Kind: FieldString, VisibleWhen: gate("supermemory")},
			{Name: "supermemory.api_key", Label: "Supermemory API key",
				Kind: FieldSecret, VisibleWhen: gate("supermemory")},
			{Name: "supermemory.user_id", Label: "Supermemory user ID",
				Kind: FieldString, VisibleWhen: gate("supermemory")},

			// Hindsight
			{Name: "hindsight.base_url", Label: "Hindsight base URL",
				Kind: FieldString, VisibleWhen: gate("hindsight")},
			{Name: "hindsight.api_key", Label: "Hindsight API key",
				Kind: FieldSecret, VisibleWhen: gate("hindsight")},
			{Name: "hindsight.bank_id", Label: "Hindsight bank ID",
				Kind: FieldString, VisibleWhen: gate("hindsight")},
			{Name: "hindsight.budget", Label: "Hindsight budget",
				Kind: FieldEnum, Enum: []string{"low", "mid", "high"},
				VisibleWhen: gate("hindsight")},

			// RetainDB
			{Name: "retaindb.base_url", Label: "RetainDB base URL",
				Kind: FieldString, VisibleWhen: gate("retaindb")},
			{Name: "retaindb.api_key", Label: "RetainDB API key",
				Kind: FieldSecret, VisibleWhen: gate("retaindb")},
			{Name: "retaindb.project", Label: "RetainDB project",
				Kind: FieldString, VisibleWhen: gate("retaindb")},
			{Name: "retaindb.user_id", Label: "RetainDB user ID",
				Kind: FieldString, VisibleWhen: gate("retaindb")},

			// OpenViking
			{Name: "openviking.endpoint", Label: "OpenViking endpoint",
				Kind: FieldString, VisibleWhen: gate("openviking")},
			{Name: "openviking.api_key", Label: "OpenViking API key",
				Kind: FieldSecret, VisibleWhen: gate("openviking")},

			// Byterover (local CLI wrapper, no api_key)
			{Name: "byterover.brv_path", Label: "Byterover brv CLI path",
				Kind: FieldString, VisibleWhen: gate("byterover")},
			{Name: "byterover.cwd", Label: "Byterover working directory",
				Kind: FieldString, VisibleWhen: gate("byterover")},

			// Holographic is a placeholder — no fields.
		},
	})
}
```

- [ ] **Step 4: Run descriptor tests to verify they pass**

Run: `go test ./config/descriptor -run "TestMemory" -v`
Expected: PASS — all 5 tests.

- [ ] **Step 5: Run full test suite to verify no regressions**

Run: `go test ./...`
Expected: PASS. Particular watch-item: `TestHandleConfigGet_RedactsSecretFields` in `api/handlers_config_test.go` picks up the new `memory` section. Since none of its test configs populate `memory`, the walk short-circuits at `walkPath(blob["memory"], "honcho.api_key")` — but `blob["memory"]` is nil, so the outer `m[sec.Key].(map[string]any)` cast returns `ok=false` and the section is skipped entirely. No new assertions needed.

- [ ] **Step 6: Commit**

```bash
git add config/descriptor/memory.go config/descriptor/memory_test.go
git commit -m "feat(config/descriptor): Memory section — 8 backends gated by provider"
```

---

### Task 6: Smoke flow documentation

**Files:**
- Create: `docs/smoke/memory.md`

**Context:** Every recent feature (Telegram proxy, Stage 4f drag-reorder) has a matching smoke flow doc in `docs/smoke/`. This one gives operators a six-step verification walk for Memory.

- [ ] **Step 1: Create the smoke flow**

Create `docs/smoke/memory.md`:

```markdown
# Memory config smoke flow

**Prereq:** hermind web running (`hermind web`), a test `config.yaml`
without a `memory:` block (or with `memory: {}`).

1. Open `http://127.0.0.1:<port>/web` in a browser.
2. Click the **Memory** group in the sidebar. The main panel shows the
   Memory section with a single **Provider** dropdown — no other fields.
3. Pick **honcho** from the provider dropdown. Four new fields appear:
   base URL, API key (secret), workspace, peer.
4. Fill **API key** = `test-honcho-key` and **workspace** = `demo`.
   Click Save.
5. Reload the page. The workspace input still reads `demo`; the API key
   input renders blank (redaction is working on the GET path).
6. Without retyping the API key, click Save again. Open `config.yaml` on
   disk — `memory.honcho.api_key` is still `test-honcho-key`
   (preservation is working on the PUT path).
7. Change the provider dropdown to **mem0**. The Honcho fields vanish;
   Mem0's `user_id`, `base_url`, `api_key` appear. The prior Honcho
   values remain in `config.yaml` under `memory.honcho` — they're just
   hidden because `visible_when` gates them on the provider discriminator.
8. Change the provider to the blank option at the top of the dropdown.
   All backend-specific fields disappear. Save. On-disk `memory.provider`
   is now empty (or the whole `memory:` block is omitted by `omitempty`).

**Regression watch:**
- Switching provider should NOT clobber the other backend's fields in
  `config.yaml` (keeps partial credentials around).
- Refreshing after saving an API key must show the secret input as
  blanked out, not as the literal key (redaction sanity check).
- Typing a new API key and saving must persist the new value, not the
  prior one (preservation only fills blanks).
```

- [ ] **Step 2: Commit**

```bash
git add docs/smoke/memory.md
git commit -m "docs(smoke): Memory config smoke flow"
```

---

## Verification (after all tasks)

- [ ] Run the full Go test suite — every package still passes.

```bash
go test ./...
```

- [ ] Run the full frontend test suite.

```bash
cd web && bunx vitest run
```

- [ ] Manually run the smoke flow in `docs/smoke/memory.md` against a running `hermind web`.

- [ ] Spot-check: open the web UI, expand the Memory group in the sidebar — it now shows **Memory** as a clickable row (no "Coming soon" text). Clicking it brings up the form.