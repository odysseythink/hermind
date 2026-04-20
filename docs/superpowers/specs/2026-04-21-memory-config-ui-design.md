# Memory Config UI — Design

**Goal:** Fill the empty "Memory" group in the web config shell with a full form for `config.MemoryConfig`, covering all 8 external-memory backends (Honcho, Mem0, Supermemory, Hindsight, RetainDB, OpenViking, Byterover, Holographic).

**Why now:** The Memory group in the sidebar renders "Coming soon — stage 5" because no descriptor is registered with `GroupID: "memory"`. `config.MemoryConfig` already exists in `config/config.go`, and the Python hermes CLI editor already knows the field layout (`config/editor/schema.go`). The web UI is the only layer missing. Shipping the form lets operators configure memory without hand-editing `config.yaml`.

**Scope:** Memory group only. Skills group (also empty) gets its own spec — see Open questions. `BrowserConfig`, which has a similar nested shape, is out of scope but the infrastructure this ships makes migrating it trivial.

---

## What this does NOT ship

- **Skills UI.** Same-shaped problem (empty group, needs a form), but skills is a toggle-list against an auto-discovered catalog, not a form. Separate design.
- **BrowserConfig migration.** `config.BrowserConfig` has the same nested shape (`browser.browserbase.api_key`) and will reuse this infrastructure, but migrating it is follow-up.
- **Validation beyond `Required`.** No URL format checks, no reachable-endpoint probe, no API-key format check. The existing field system ships what every other section gets.
- **Per-backend "test connection" button.** No equivalent of the gateway `/test` endpoint. Operators verify by saving config and running the agent. A reachability probe is a follow-up stage.
- **Migrating `memory.provider` to a richer enum component.** The existing `EnumSelect` renders a plain `<select>` — good enough. Pretty provider icons and descriptions are UI polish for later.
- **A new `SectionShape`.** No `ShapeDiscriminated` or similar. The discriminator pattern (one `FieldEnum` + sibling fields gated by `VisibleWhen`) already works — `storage.go` uses it for `driver=sqlite|postgres`. We extend it to nested paths without inventing a shape.

---

## Architecture

Two concerns:

1. **Dotted-path infrastructure** (one-time, ~40 lines of diff) — teaches the descriptor renderer, the state reducer, and the backend secret helpers to walk dotted field names like `honcho.api_key`. Fields without dots keep working identically; nothing else in the codebase changes behavior.
2. **Memory descriptor** (~95 lines, one file) — a single `Section{Key: "memory", Shape: ShapeMap}` with a `provider` `FieldEnum` discriminator plus per-backend fields using dotted names, each gated by `VisibleWhen`.

The division of labor is intentional: the infrastructure change is small enough to land with the descriptor, but it's reusable for `BrowserConfig` and any future nested config.

---

## Infrastructure: dotted-path support

### New: `web/src/util/path.ts`

Two pure functions, no dependencies:

```ts
export function getPath(obj: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>(
    (o, k) => (o as Record<string, unknown> | undefined)?.[k],
    obj,
  );
}

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

`setPath` is immutable — returns a new object, matches the reducer's copy-on-write style. Non-dotted paths behave exactly like the current spread pattern. Intermediate keys are created lazily as `{}` when missing.

### Modified: `web/src/components/ConfigSection.tsx`

Three call sites switch from direct-index to path-walk:

```ts
// before
const current = asString(value[f.name]);
const original = asString(originalValue[f.name]);
// after
const current = asString(getPath(value, f.name));
const original = asString(getPath(originalValue, f.name));

// before
return String(value[f.visible_when.field]) === String(f.visible_when.equals);
// after
return String(getPath(value, f.visible_when.field)) === String(f.visible_when.equals);
```

The `onChange` signature is unchanged — it still passes the dotted name up to the parent via `onFieldChange(f.name, v)`. The reducer handles the path walk on the write side.

### Modified: `web/src/state.ts` — `edit/config-field` reducer

Single line change:

```ts
// before
[action.sectionKey]: { ...prev, [action.field]: action.value },
// after
[action.sectionKey]: setPath(prev, action.field, action.value),
```

Flat names behave identically; dotted names walk down.

### Modified: `api/handlers_config.go`

Both `redactSectionSecrets` and `preserveSectionSecrets` walk dotted paths for the `ShapeMap` branch only (the `ShapeKeyedMap` and `ShapeList` branches don't need it — they nest via instance keys, which is a different axis).

Shared helper:

```go
// walkPath follows dotted keys down m, returning the leaf's parent map and
// the final key. ok=false means the path is missing at some intermediate
// level; callers skip that field.
func walkPath(m map[string]any, path string) (map[string]any, string, bool) {
    keys := strings.Split(path, ".")
    cur := m
    for i, k := range keys {
        if i == len(keys)-1 {
            return cur, k, true
        }
        next, ok := cur[k].(map[string]any)
        if !ok {
            return nil, "", false
        }
        cur = next
    }
    return nil, "", false // unreachable
}
```

`redactSectionSecrets` blanks `parent[leaf]` when the field is present. `preserveSectionSecrets` copies `curParent[leaf]` → `updParent[leaf]` when the updated leaf is blank.

Fields without dots go through `walkPath` and come back as `(m[sec.Key], fieldName, true)` — same behavior as before, no special case needed.

---

## Memory descriptor: `config/descriptor/memory.go`

One registration. Provider selector is the `FieldEnum` discriminator; every per-backend field has `VisibleWhen: &Predicate{Field: "provider", Equals: "<backend>"}`.

### Fields

| Name | Kind | VisibleWhen | Notes |
|---|---|---|---|
| `provider` | Enum | *(always)* | Enum: `["", "honcho", "mem0", "supermemory", "hindsight", "retaindb", "openviking", "byterover", "holographic"]`. `""` = no external memory. |
| `honcho.base_url` | String | `provider=honcho` | |
| `honcho.api_key` | Secret | `provider=honcho` | |
| `honcho.workspace` | String | `provider=honcho` | |
| `honcho.peer` | String | `provider=honcho` | |
| `mem0.base_url` | String | `provider=mem0` | |
| `mem0.api_key` | Secret | `provider=mem0` | |
| `mem0.user_id` | String | `provider=mem0` | |
| `supermemory.base_url` | String | `provider=supermemory` | |
| `supermemory.api_key` | Secret | `provider=supermemory` | |
| `supermemory.user_id` | String | `provider=supermemory` | |
| `hindsight.base_url` | String | `provider=hindsight` | |
| `hindsight.api_key` | Secret | `provider=hindsight` | |
| `hindsight.bank_id` | String | `provider=hindsight` | |
| `hindsight.budget` | Enum | `provider=hindsight` | Enum: `["low", "mid", "high"]` |
| `retaindb.base_url` | String | `provider=retaindb` | |
| `retaindb.api_key` | Secret | `provider=retaindb` | |
| `retaindb.project` | String | `provider=retaindb` | |
| `retaindb.user_id` | String | `provider=retaindb` | |
| `openviking.endpoint` | String | `provider=openviking` | |
| `openviking.api_key` | Secret | `provider=openviking` | |
| `byterover.brv_path` | String | `provider=byterover` | |
| `byterover.cwd` | String | `provider=byterover` | |

22 fields total (plus the `provider` selector). 6 secrets — one `api_key` per backend except Byterover (CLI wrapper, no API) and Holographic (placeholder, no fields at all). Holographic selected shows the selector with nothing below, which is correct.

### Default / required

- `provider` is not `Required` — `""` is a valid, default-on-boot state.
- Individual backend fields aren't `Required` either. A partially-configured Honcho block (workspace set, api_key blank) is a valid in-progress edit; runtime memprovider code handles the missing-credential case. Pinning `Required` on `api_key` would block users from saving an in-progress config.

---

## Testing

### Unit — `web/src/util/path.test.ts` (new)

6 cases:

1. `getPath(obj, "a")` returns a flat field.
2. `getPath(obj, "a.b")` returns a nested field.
3. `getPath(obj, "a.b")` returns `undefined` when `a` is missing.
4. `setPath(obj, "a", v)` writes a flat field.
5. `setPath(obj, "a.b", v)` writes a nested field, creating intermediate `{}`.
6. `setPath` returns a new object — input is never mutated.

### Component — `web/src/components/ConfigSection.test.tsx` (extend)

One new test: given a section with `fields: [{ name: "provider", kind: "enum", enum: ["honcho"] }, { name: "honcho.api_key", kind: "secret", visible_when: { field: "provider", equals: "honcho" } }]` and `value: { provider: "honcho", honcho: { api_key: "xxx" } }`, the SecretInput renders with `"xxx"` and typing dispatches `onFieldChange("honcho.api_key", <new-value>)`.

### Reducer — `web/src/state.test.ts` (extend)

One new `edit/config-field` case: `dispatch({ type: "edit/config-field", sectionKey: "memory", field: "honcho.api_key", value: "k" })` on a state with `memory: { provider: "honcho", honcho: { workspace: "w" } }` produces `memory: { provider: "honcho", honcho: { workspace: "w", api_key: "k" } }`. The `workspace` sibling survives; the write is non-destructive.

### Backend — `api/handlers_config_test.go` (extend)

Two cases:

1. `redactSectionSecrets` blanks `memory.honcho.api_key` when the Memory descriptor is registered and the map contains a non-empty value.
2. `preserveSectionSecrets` copies `memory.honcho.api_key` from current to updated when updated's value is blank and current's is set.

### Descriptor — `config/descriptor/memory_test.go` (new)

Pins the schema. Asserts:

- Section `"memory"` is registered with `GroupID: "memory"` and `Shape: ShapeMap`.
- The `provider` field is `FieldEnum` with the 9 expected values (8 backends + `""`).
- Every field matching `<backend>.api_key` has `Kind: FieldSecret`.
- Every non-provider field has a non-nil `VisibleWhen` with `Field: "provider"` (spot check via iteration).

Prevents accidental schema drift during future refactors.

---

## Smoke flow: `docs/smoke/memory.md` (new)

```
# Memory config smoke flow

Prereq: hermind web running, a test config.yaml without a `memory:` block.

1. Open /web. Click the Memory group in the sidebar — see a blank form with
   just the provider selector.
2. Pick "honcho" from the provider dropdown. Verify workspace, peer,
   base_url, and api_key fields appear.
3. Fill api_key = "test_honcho_key" and workspace = "demo". Save.
4. Reload the page. Confirm workspace round-trips as "demo" and api_key
   renders as a blanked-out secret input (redaction worked).
5. Save the page again without editing api_key. Open config.yaml on disk —
   the prior api_key value is preserved (preservation worked).
6. Change provider to "mem0". Honcho fields disappear, mem0 fields
   (user_id, base_url, api_key) appear. The saved Honcho values stay in
   config.yaml under memory.honcho but aren't visible.
7. Change provider to "" (blank). All fields disappear; config.yaml on
   next save has memory: {} (or the block is omitted via omitempty).
```

---

## File manifest

**Created:**
- `config/descriptor/memory.go` — the Memory section registration (~95 lines).
- `config/descriptor/memory_test.go` — schema pin (~50 lines).
- `web/src/util/path.ts` — getPath / setPath helpers (~15 lines).
- `web/src/util/path.test.ts` — 6 unit cases.
- `docs/smoke/memory.md` — operator verification flow.

**Modified:**
- `web/src/components/ConfigSection.tsx` — three call sites switch to `getPath`.
- `web/src/state.ts` — `edit/config-field` reducer uses `setPath`.
- `api/handlers_config.go` — `redactSectionSecrets` and `preserveSectionSecrets` learn dotted paths for the ShapeMap branch (via shared `walkPath` helper).
- `web/src/components/ConfigSection.test.tsx` — extend with one dotted-path case.
- `web/src/state.test.ts` — extend `edit/config-field` with one dotted-path case.
- `api/handlers_config_test.go` — two dotted-path cases.

**Untouched:**
- `config/config.go` — no struct changes; existing yaml tags drive the shape.
- `config/editor/schema.go` — the Python-style CLI editor schema already covers memory. Untouched.
- `web/src/components/shell/Sidebar.tsx`, `SectionList.tsx` — no changes; registering a section under `GroupID: "memory"` auto-appears via `configSections.filter(s => s.group_id === g.id)`.

---

## Open questions carried to later stages

- **Skills UI.** Same "empty group" problem. Skills needs a toggle-list against the auto-discovered skill catalog, which doesn't fit any existing `SectionShape`. Likely a custom component (`SkillsSidebar` / `SkillsEditor`) like `ModelsSidebar` — separate spec.
- **BrowserConfig migration.** `browser.browserbase.api_key` / `browser.camofox.base_url` have the same shape. One follow-up PR: register a `browser` descriptor using the dotted-path infra this ships.
- **Per-backend test/reachability probe.** None of the existing sections have a "test" button (only gateway platforms do). If Memory users want one, it's a larger infra question — how does ShapeMap express a test endpoint? Punt.
- **Discriminated-shape formalism.** If three or more sections end up using "one selector enum + N branches gated by VisibleWhen", consider introducing `ShapeDiscriminated` that formally represents the pattern and lets the frontend render a tabbed or wizard layout. Right now: storage, memory, browser — three uses, enough to consider it, but not required for memory to ship.
