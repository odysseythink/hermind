# Web Config Schema Infrastructure — Stage 4a Design

**Goal:** Ship the Default Model picker (`config.Model`) and the Auxiliary provider editor (`config.AuxiliaryConfig`), in the process introducing scalar-section support to the descriptor system.

**Sub-stage context:** Stage 4 as a whole lights up the Models group (default model, providers, fallback providers) and the Auxiliary field under Runtime. The "everything at once" scope is too large for one spec, so Stage 4 is split:

- **4a (this spec)** — `model` scalar + `auxiliary` single-struct. Introduces scalar-section infra. No new API endpoints.
- **4b (future)** — `providers` map editor + `POST /api/providers/models` fetch-models endpoint + connection test. Introduces map-valued-section infra.
- **4c (future)** — `fallback_providers` ordered list. Introduces list-valued-section infra.

The sub-stages are loosely coupled. 4a is fully independent; 4b and 4c can ship in either order after 4a.

**Non-goals for 4a:**
- Anything that depends on the `providers` map editor (dynamic model autocomplete, fetch-models button, connection test).
- A list-field editor (postponed to 4c).
- A nested-section editor for `agent.compression`, `terminal.docker_volumes`, or similar (separate stage).

---

## Architecture

Stage 2 established a flat descriptor system where each registered `Section` maps to a top-level YAML key whose value is a `map[string]any`. That works for `storage`, `agent`, `terminal`, `logging`, `metrics`, `tracing`, and — in 4a — `auxiliary`. It does not fit `config.Model`, which is a top-level scalar string.

The fix is a new `Section.Shape` discriminant:

```go
type SectionShape int

const (
    ShapeMap    SectionShape = iota // default — m[sec.Key] is map[string]any
    ShapeScalar                     // m[sec.Key] is a scalar; Fields has exactly 1 entry
)
```

The zero value is `ShapeMap`, so every existing descriptor is unaffected. Four backend touchpoints branch on `Shape`:

1. `redactSectionSecrets` / `preserveSectionSecrets` in `api/handlers_config.go`. For `ShapeScalar` sections, operate on `m[sec.Key]` directly. The `model` field isn't a secret so the redact path is a no-op in practice; the branch exists so a future scalar-secret section doesn't silently fall through.
2. `handleConfigSchema` in `api/handlers_config_schema.go`. The DTO gains an optional `shape` string field; `omitempty` keeps existing section responses byte-identical when `Shape == ShapeMap`.
3. `TestSectionInvariants` in `config/descriptor/descriptor_test.go`. When `Shape == ShapeScalar`, enforce `len(Fields) == 1`.
4. Frontend — see below.

On the frontend, `ContentPanel.tsx` is the branching boundary. `<ConfigSection>` stays generic; it renders `section.fields[0]` exactly the same way it renders storage's driver field. The scalar-vs-map decision lives one level up in ContentPanel, which wraps the raw scalar into `{ [field.name]: scalar }` for ConfigSection and unwraps on the way back.

New: `provider/factory/types.go` exports `func Types() []string` returning a sorted slice of registered provider-type names. The `auxiliary` descriptor's `provider` field uses this as its `Enum` source so the list stays in sync as new provider packages land.

---

## Backend components

### §A Descriptor infra (`config/descriptor/descriptor.go`)

```go
type SectionShape int

const (
    ShapeMap SectionShape = iota
    ShapeScalar
)

type Section struct {
    Key     string
    Label   string
    Summary string
    GroupID string
    Shape   SectionShape
    Fields  []FieldSpec
}
```

`Register`, `Get`, and `All` are unchanged. The test suite grows one invariant:

```go
// inside TestSectionInvariants:
if s.Shape == ShapeScalar && len(s.Fields) != 1 {
    t.Errorf("section %q: ShapeScalar requires exactly 1 field, got %d", s.Key, len(s.Fields))
}
```

### §B `provider/factory/types.go`

```go
// Types returns the provider-type strings the factory knows about, sorted.
// Used by the auxiliary descriptor's provider-enum field so the UI stays
// in sync when new provider packages are added.
func Types() []string {
    out := make([]string, 0, len(registry))
    for k := range registry {
        out = append(out, k)
    }
    sort.Strings(out)
    return out
}
```

Implementation detail: `registry` is the existing map inside `factory.go` that `New` dispatches on. If it isn't already a package-level map, this task extracts it.

### §C `config/descriptor/model.go`

```go
func init() {
    Register(Section{
        Key:     "model",
        Label:   "Default model",
        Summary: "Model used when a request doesn't pin one explicitly.",
        GroupID: "models",
        Shape:   ShapeScalar,
        Fields: []FieldSpec{
            {
                Name:     "model",
                Label:    "Model",
                Help:     "Provider-qualified id, e.g. anthropic/claude-opus-4-7.",
                Kind:     FieldString,
                Required: true,
            },
        },
    })
}
```

### §D `config/descriptor/auxiliary.go`

```go
func init() {
    Register(Section{
        Key:     "auxiliary",
        Label:   "Auxiliary provider",
        Summary: "Secondary provider for compression, vision, and background tasks. Leave all fields blank to reuse the main provider.",
        GroupID: "runtime",
        Fields: []FieldSpec{
            {Name: "provider", Label: "Provider", Kind: FieldEnum,
                Help: "Provider factory. Leave blank to reuse the main provider.",
                Enum: factory.Types()},
            {Name: "base_url", Label: "Base URL", Kind: FieldString},
            {Name: "api_key", Label: "API key", Kind: FieldSecret},
            {Name: "model", Label: "Model", Kind: FieldString,
                Help: "Provider-qualified id; optional — falls back to the main provider's default."},
        },
    })
}
```

Shape defaults to `ShapeMap`. No field is `Required` because "all blank = reuse main provider" is a valid state.

**`factory.Types()` evaluation timing.** Go `init()` functions run in package-import order. `config/descriptor/auxiliary.go` imports `provider/factory`, so `factory.init()` runs first and the registry is populated before `Register(Section{…})` is called. Verified on the Go spec: "Package initialization happens in a single goroutine, sequentially, one package at a time."

### §E Backend DTO + handlers

**`api/dto.go`** adds to `ConfigSectionDTO`:

```go
Shape string `json:"shape,omitempty"` // "scalar" when Section.Shape == ShapeScalar
```

**`api/handlers_config_schema.go`** — when building the DTO, set `Shape` for scalar sections:

```go
if sec.Shape == descriptor.ShapeScalar {
    dto.Shape = "scalar"
}
```

`ShapeMap` sections serialize with no `shape` key (existing byte-level contract preserved).

**`api/handlers_config.go`** — `redactSectionSecrets` and `preserveSectionSecrets` both walk `descriptor.All()`. Each gains a `Shape` branch:

```go
for _, sec := range descriptor.All() {
    if sec.Shape == descriptor.ShapeScalar {
        // Scalar: m[sec.Key] is the value itself, not a map. In 4a there
        // are no scalar secrets (model isn't one), so both redact and
        // preserve are no-ops; the branch exists so a future scalar-secret
        // descriptor doesn't silently fall through the map path.
        continue
    }
    // existing map path
    blob, ok := m[sec.Key].(map[string]any)
    // …
}
```

---

## Frontend components

### §F Schema + state

**`web/src/api/schemas.ts`** — add an optional shape field:

```ts
export const ConfigSectionSchema = z.object({
    key: z.string(),
    label: z.string(),
    summary: z.string().optional(),
    group_id: z.string(),
    shape: z.enum(['map', 'scalar']).optional(), // default-undefined = map
    fields: z.array(ConfigFieldSchema),
});
```

**`web/src/state.ts`** — add one action:

```ts
| { type: 'edit/config-scalar'; sectionKey: string; value: unknown }
```

Reducer:

```ts
case 'edit/config-scalar':
    return {
        ...state,
        config: { ...state.config, [action.sectionKey]: action.value },
    };
```

Dirty-tracking and Save flow piggyback on the existing config object diff — no change needed at the `totalDirtyCount` / Save-button layer.

### §G `ContentPanel.tsx`

Branch on `section.shape` inside the existing `activeSubKey` handling:

```tsx
if (section.shape === 'scalar') {
    const scalar = (props.config as Record<string, unknown>)[section.key];
    const originalScalar = (props.originalConfig as Record<string, unknown>)[section.key];
    const field = section.fields[0];
    return (
        <ConfigSection
            section={section}
            value={{ [field.name]: scalar }}
            originalValue={{ [field.name]: originalScalar }}
            onFieldChange={(_name, v) => props.onConfigScalar(section.key, v)}
        />
    );
}
// existing map path
const value = (props.config as Record<string, unknown>)[section.key] as Record<string, unknown> | undefined;
// …
```

The `onConfigScalar` prop threads down from `App.tsx` the same way `onConfigField` already does, dispatching `edit/config-scalar`.

**`<ConfigSection>` itself is unchanged.** A single-field section already renders correctly; the scalar-vs-map decision is a data-shape concern and belongs at the ContentPanel boundary.

### §H `sections.ts`

Two new entries. Declaration order inside the `runtime` group ramps by complexity:

```ts
export const SECTIONS: readonly SectionDef[] = [
    // runtime
    { key: 'storage',   groupId: 'runtime', plannedStage: 'done' },
    { key: 'agent',     groupId: 'runtime', plannedStage: 'done' },
    { key: 'auxiliary', groupId: 'runtime', plannedStage: 'done' }, // new
    { key: 'terminal',  groupId: 'runtime', plannedStage: 'done' },
    // observability
    { key: 'logging',   groupId: 'observability', plannedStage: 'done' },
    { key: 'metrics',   groupId: 'observability', plannedStage: 'done' },
    { key: 'tracing',   groupId: 'observability', plannedStage: 'done' },
    // models
    { key: 'model',     groupId: 'models', plannedStage: 'done' }, // new
] as const;
```

The existing declaration-order tests are updated to include the two new keys.

---

## Data flow

### Reading config

1. Browser requests `GET /api/config/schema`. Response includes `{key: "model", shape: "scalar", fields: [{name: "model", kind: "string", …}]}` and `{key: "auxiliary", fields: [...]}` (no `shape` key, defaults to map).
2. Browser requests `GET /api/config`. Response includes `"model": "anthropic/claude-opus-4-6"` (scalar) and `"auxiliary": {"provider": "...", "api_key": ""}` (map; `api_key` blanked by `redactSectionSecrets`).
3. React stores both verbatim in `state.config`.
4. Navigating to `#models/model`: `ContentPanel` detects `shape === 'scalar'`, wraps the raw string as `{model: "anthropic/claude-opus-4-6"}`, renders `<ConfigSection>`.
5. Navigating to `#runtime/auxiliary`: existing map path, unchanged.

### Editing & saving

1. User types in the Model field: `<TextInput>` calls `onChange('anthropic/claude-opus-4-7')`. ConfigSection calls `onFieldChange('model', 'anthropic/claude-opus-4-7')`. ContentPanel's scalar branch ignores the field name and calls `props.onConfigScalar('model', 'anthropic/claude-opus-4-7')`. State reducer sets `state.config.model = 'anthropic/claude-opus-4-7'` (raw scalar, no wrapper).
2. User clicks Save. Existing save flow: `PUT /api/config` sends the whole `state.config` as JSON. Backend `yaml.Unmarshal` maps `"model": "anthropic/claude-opus-4-7"` directly to `config.Config.Model` via the `yaml:"model"` tag.
3. `preserveSectionSecrets` iterates sections; the scalar branch skips without touching `model`; the map branch runs for `auxiliary` and preserves `api_key` if submitted blank.
4. YAML is written to disk. `model: anthropic/claude-opus-4-7` appears as a plain top-level scalar, not wrapped.

### Auxiliary "all blank = reuse main provider"

When the user clears every auxiliary field:
- `auxiliary: {provider: "", base_url: "", api_key: "", model: ""}` goes up in the PUT body.
- Backend `preserveSectionSecrets` sees `api_key: ""` and restores the stored value if there was one; if the user truly wants to clear the stored key, they have to edit `config.yaml` directly (Stage 2 preserve-secrets semantics).
- The serialized YAML drops the whole block because every sub-field has `yaml:"...,omitempty"`.
- At runtime, `provider` package reads an empty `AuxiliaryConfig` as "reuse main" per existing logic (`provider/auxiliary.go`).

No new clear-secret UX is introduced in 4a; that's general Stage-2 infra whose behavior is already documented.

---

## Error handling

- **`ShapeScalar` section registered with !=1 fields** — `TestSectionInvariants` fails at test time. Registration at runtime still succeeds; the invariant is the gate.
- **`factory.Types()` returns empty** — unlikely (every provider package calls `factory.Register` in its init), but if it does the auxiliary descriptor's `Enum` is empty and the dropdown renders zero options. `TestSectionInvariants` already rejects `FieldEnum` with empty `Enum`, so this blows up at test time.
- **Scalar section receives malformed data on boot** — e.g., backend sends `"model": {"unexpected": "object"}`. ContentPanel's scalar branch does `asString(undefined)` (becomes `""`) and the editor renders an empty Model field. Save flow writes whatever the user types. No crash, no silent corruption.
- **DTO mismatch between backend and frontend** — z.enum on `shape` rejects unknown values. Backend is the source of truth; a typo would fail Zod parse at boot and surface as a user-visible error.

---

## Testing

### Go

- `config/descriptor/model_test.go` — registration, `Shape == ShapeScalar`, `len(Fields) == 1`, Required true, Help text non-empty.
- `config/descriptor/auxiliary_test.go` — registration, 4 fields of expected kinds, `api_key` is `FieldSecret`, `provider.Enum` non-empty (sanity — asserts `factory.Types()` wired up; does NOT pin the full list since that churns as providers land).
- `config/descriptor/descriptor_test.go` — extend `TestSectionInvariants` with the ShapeScalar-requires-1-field check.
- `provider/factory/types_test.go` — `Types()` returns sorted, non-empty, includes `anthropic` and `openai` (sanity floor).
- `api/handlers_config_schema_test.go` — new `TestConfigSchema_IncludesStage4aSections` asserting `model` has `shape: "scalar"` under `models`, `auxiliary` has no `shape` key under `runtime`.
- `api/handlers_config_test.go` — GET `/api/config` with `model: "x"` round-trips unwrapped; GET with `auxiliary.api_key: "secret"` blanks it; PUT with `auxiliary.api_key: ""` preserves the stored key.

### Web

- `web/src/shell/sections.test.ts` — declaration-order assertion extended to include `auxiliary` in runtime at position 2 (between agent and terminal) and `model` in models group.
- `web/src/components/shell/ContentPanel.test.tsx` — new describe block for scalar section routing: given a `ShapeScalar` section + raw-string config slice, renders `<ConfigSection>` with wrapped value; typing fires `onConfigScalar(sectionKey, newValue)`.
- `web/src/state.test.ts` — `edit/config-scalar` action sets the scalar under the given key.
- `web/src/api/schemas.test.ts` — parses `shape: "scalar"` and tolerates `shape` absent.

### Smoke doc

Append `## Stage 4a · Default model + Auxiliary` to `docs/smoke/web-config.md`:

- `#models/model` renders a single Model text field. Typing `anthropic/claude-opus-4-7` and Saving writes `model: anthropic/claude-opus-4-7` as a plain top-level YAML scalar (not a nested object). Verify with `grep '^model:' ~/.hermind/config.yaml`.
- `#runtime/auxiliary` renders four fields: Provider (enum populated from `provider/factory`), Base URL, API key (`FieldSecret`), Model. API key is blanked on GET and preserved on empty PUT (same as `storage.postgres_url`).
- Leave all four auxiliary fields blank. Save. Grep YAML — the `auxiliary:` block should not appear (every sub-field is `omitempty`). Main provider remains in use.

---

## Delivery plan

Nine tasks, each self-contained and test-first:

1. **Descriptor `Shape` infra.** Add `SectionShape` enum + `Section.Shape` field + invariant extension + update of existing tests. Pure infra, no new descriptors.
2. **`factory.Types()` accessor.** Extract the provider registry into a package-level map if it isn't already one; add sorted accessor; test.
3. **Backend plumbing.** `redact`/`preserve`/schema DTO branches on `Shape`. Includes new DTO field + JSON tag.
4. **Frontend schema + state.** `ConfigSectionShapeSchema`, `edit/config-scalar` reducer, `ContentPanel` scalar branch, `onConfigScalar` prop plumbing in `App.tsx`.
5. **`model` descriptor + test.** Exercises the ShapeScalar code path end-to-end.
6. **`auxiliary` descriptor + test.** Depends on Task 2 (Types). Regular ShapeMap section.
7. **API schema contract test.** Pins both sections in `/api/config/schema`.
8. **`sections.ts` + test update.** Append two rows; extend declaration-order tests.
9. **Gauntlet + webroot rebuild + smoke doc append.** Mirrors Stage 3 Task 9 + 10.

No cross-task dependencies except Task 6 → Task 2.

---

## What this does NOT change

- Existing Stage 2/3 descriptors (storage, agent, terminal, logging, metrics, tracing) — zero code change; their JSON responses stay byte-identical because `Shape` defaults to `ShapeMap` and serializes with `omitempty`.
- Secret round-trip behavior for platform instances or existing sections.
- Dirty-tracking / Save-button logic.
- `<ConfigSection>` itself (the scalar wrapping happens in ContentPanel).
- CLI workflow (`./bin/hermind run`, `hermind cron`, etc.) — they read the same YAML that was always written.

---

## Open questions carried into 4b / 4c

- **Scalar-secret sections.** None in 4a. If a future descriptor needs it, `redactSectionSecrets`'s scalar branch will need to blank the value; hooks are in place, behavior defers.
- **Nested sections.** `agent.compression`, `terminal.docker_volumes`, and similar nested or list-valued sub-trees remain CLI-only. 4a doesn't address them; 4b introduces map-valued section support which is the natural home for maps-of-structs like `providers`; 4c introduces lists.
- **Model picker UX.** Plain string in 4a. 4b's fetch-models endpoint will enable a dynamic `<datalist>` autocomplete on both `model` and `auxiliary.model`. Design: same `FieldString` kind + an optional `"datalist_source": string` DTO field pointing at a provider key. Deferred to 4b so we can validate the fetch-models flow end-to-end first.
