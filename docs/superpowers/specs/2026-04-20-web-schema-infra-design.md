# Web Config Schema Infrastructure — Stage 2 Design

**Date:** 2026-04-20
**Topic:** Generic schema infrastructure for non-platform config sections, with Storage as the vertical slice
**Status:** Approved

## Context

Stage 1 of the web-config expansion (`docs/superpowers/specs/2026-04-20-web-shell-rewrite-design.md`) replaced the IM-only Vite/React UI with a seven-group shell. Only the Gateway group ships a working editor; the other six groups render a "Coming soon — stage N" placeholder.

Stages 3–7 each need to render editors for specific `config.Config` fields (Model, Agent, Terminal, Storage, Providers, Memory, Skills, MCP, Browser, Cron, Logging, Metrics, Tracing). Rather than hand-code a React editor per section, Stage 2 builds the shared schema infrastructure all later stages consume:

1. A Go descriptor package that advertises each section's fields, analogous to the existing `gateway/platforms.Descriptor` used for IM platforms.
2. A `GET /api/config/schema` endpoint that serves those descriptors.
3. A generic React `ConfigSection` component that renders a section from its descriptor, wired into the shell state and routing.

To avoid shipping a DSL that doesn't fit the real workload, Stage 2 includes exactly one real section end-to-end: **Storage**. Storage is the smallest section that forces a discriminated-union pattern (driver selects which field is active) — the shape stages 5 (Memory) and 7 (MCP/Browser/Cron) will need at scale. Solving it here avoids a re-plumb in those stages.

## Sub-project Sequence (for context)

1. Stage 1 — Shell / navigation rewrite (shipped)
2. **Stage 2 — Schema infrastructure + Storage vertical slice** (this spec)
3. Stage 3 — Simple sections (Model / Agent / Terminal / Logging / Metrics / Tracing)
4. Stage 4 — Providers editor (multi-type + "fetch models" button)
5. Stage 5 — Memory editor (multi-backend branching)
6. Stage 6 — Skills editor (enable/disable + platform overrides)
7. Stage 7 — MCP / Browser / Cron
8. Stage 8 — Sunset `cli/ui/webconfig/`, flip `./bin/hermind` default command

## Scope

### In Scope

- New Go package `config/descriptor/` with a `Descriptor`/`Section` type, a registry, and one `Storage` descriptor file.
- One new `FieldKind` value: `FieldFloat`. No list, map, or group kinds yet.
- `FieldSpec.VisibleWhen *Predicate` — conditional visibility against another field's value.
- `GET /api/config/schema` endpoint returning all registered sections sorted by `Key`.
- Extension of `redactSecrets` in `api/handlers_config.go` to walk every registered section and blank fields marked `FieldSecret`.
- Zod schemas + TypeScript types for the new response.
- React `ConfigSection.tsx` generic renderer + `FloatInput.tsx` field component.
- New reducer action `edit/config-field`; updated `totalDirtyCount`.
- `shell/sections.ts` registry table + `SectionList` sidebar row for non-gateway groups.
- `ContentPanel.tsx` routing: `activeGroup === 'runtime' && activeSubKey === 'storage'` → `<ConfigSection>`; other combinations continue to render `ComingSoonPanel`.
- Deep link `#runtime/storage` renders the Storage editor.
- Test coverage: Go descriptor invariants + endpoint + redact; React renderer + zod + state + sidebar + integration.

### Out of Scope

- Any section descriptor other than Storage (stage 3+).
- Discriminated-union nested structs (stage 5's Memory, stage 7's MCP) — Stage 2 only supports the flat-struct-with-VisibleWhen pattern.
- List (`[]T`) and map (`map[string]T`) field kinds.
- `POST /api/config/reveal` or any generalized reveal endpoint for non-platform secrets. `SecretInput`'s Show button is disabled for section fields in Stage 2; users can overwrite but not reveal `postgres_url`.
- Per-section state slicing or per-section reducers. The existing single `config` tree is kept.
- Merging `config/descriptor/` with `gateway/platforms/`. The two packages stay separate; their field concepts are similar but their responsibilities differ (platforms carry `Build`/`Test` closures, sections are pure schema).
- Changes to the Sidebar's default expansion behavior. Only Gateway is expanded on first load; Runtime remains collapsed. Storage is still reachable via deep link or by clicking the group's expand arrow.
- Changes to the TopBar, Footer, or Apply button wiring.

## Architecture

### Go side

```
config/descriptor/
    descriptor.go         # Section + FieldSpec + FieldKind + Predicate + registry
    descriptor_test.go    # invariants: non-empty fields, no FieldUnknown, VisibleWhen references valid field, etc.
    storage.go            # Storage Section, registered via init()
    storage_test.go       # descriptor shape matches config.StorageConfig yaml tags
```

`config/descriptor/` is a sibling of `config/`, not nested — the `config` package is the data model; `descriptor` is the UI schema layer. They avoid an import cycle because `descriptor` imports `config` (to reference default values or validation) while `config` never imports `descriptor`.

```go
package descriptor

type FieldKind int

const (
    FieldUnknown FieldKind = iota
    FieldString
    FieldInt
    FieldBool
    FieldSecret
    FieldEnum
    FieldFloat  // NEW in Stage 2
)

type Predicate struct {
    Field  string
    Equals any
}

type FieldSpec struct {
    Name        string       // yaml key: "sqlite_path"
    Label       string
    Help        string
    Kind        FieldKind
    Required    bool
    Default     any
    Enum        []string
    VisibleWhen *Predicate   // NEW
}

type Section struct {
    Key     string        // "storage" — matches yaml tag on config.Config
    Label   string
    Summary string
    GroupID string        // "runtime" — which shell group hosts this section
    Fields  []FieldSpec
}

func Register(s Section) { /* ... */ }
func Get(key string) (Section, bool) { /* ... */ }
func All() []Section { /* sorted by Key */ }
```

`storage.go` registers:

```go
func init() {
    Register(Section{
        Key:     "storage",
        Label:   "Storage",
        Summary: "Where hermind keeps conversation history and agent state.",
        GroupID: "runtime",
        Fields: []FieldSpec{
            {Name: "driver", Label: "Driver", Kind: FieldEnum,
                Enum: []string{"sqlite", "postgres"}, Required: true, Default: "sqlite"},
            {Name: "sqlite_path", Label: "SQLite path", Kind: FieldString,
                Help: "Filesystem path to the SQLite database file.",
                VisibleWhen: &Predicate{Field: "driver", Equals: "sqlite"}},
            {Name: "postgres_url", Label: "Postgres URL", Kind: FieldSecret,
                Help: "postgres://user:pass@host/db connection string.",
                VisibleWhen: &Predicate{Field: "driver", Equals: "postgres"}},
        },
    })
}
```

### REST

```
GET /api/config/schema → 200
{
  "sections": [
    {
      "key": "storage",
      "label": "Storage",
      "summary": "...",
      "group_id": "runtime",
      "fields": [
        {"name": "driver", "label": "Driver", "kind": "enum",
         "enum": ["sqlite", "postgres"], "required": true, "default": "sqlite"},
        {"name": "sqlite_path", "label": "SQLite path", "kind": "string",
         "help": "...", "visible_when": {"field": "driver", "equals": "sqlite"}},
        {"name": "postgres_url", "label": "Postgres URL", "kind": "secret",
         "help": "...", "visible_when": {"field": "driver", "equals": "postgres"}}
      ]
    }
  ]
}
```

Served by new handler `api.handleConfigSchema` (new file `api/handlers_config_schema.go`), thin wrapper over `descriptor.All()`. Registered in `api/server.go` alongside the existing `/api/platforms/schema`.

**`PUT /api/config` unchanged in shape.** The existing handler round-trips the full Config via YAML and preserves unchanged secrets via string-equal-to-previous-redacted heuristics. Stage 2 extends the redaction scan only — the save path stays identical. The frontend mutates the `config.storage` slice in place; Save ships the entire blob.

**`redactSecrets` extension:** today it only walks `m["gateway"]["platforms"][*]["options"]`. Extend it to also walk `m[sec.Key][field.Name]` for every `sec` returned by `descriptor.All()` and every `field` where `field.Kind == FieldSecret`. Silently skips unknown sections or missing keys. Preserves the existing "blank string equals 'unchanged'" contract that `PUT /api/config` already relies on.

### Frontend

```
web/src/
    api/
        schemas.ts              # ADD: ConfigFieldSchema, ConfigSectionSchema, ConfigSchemaResponseSchema
        client.ts               # ADD: getConfigSchema()
    components/
        ConfigSection.tsx       # NEW: generic section editor
        ConfigSection.test.tsx  # NEW
        fields/
            FloatInput.tsx      # NEW: float-valued NumberInput variant
    shell/
        sections.ts             # NEW: { key: 'storage', groupId: 'runtime', plannedStage: 'done' }, etc.
        sections.test.ts        # NEW
    state.ts                    # MODIFY: add 'edit/config-field' action, update totalDirtyCount, add configSections
    state.test.ts               # MODIFY
    App.tsx                     # MODIFY: boot loads configSections in parallel; ContentPanel + Sidebar receive them
    components/shell/
        ContentPanel.tsx        # MODIFY: route runtime/storage → ConfigSection
        ContentPanel.test.tsx   # MODIFY
        Sidebar.tsx             # MODIFY: render SectionList inside non-gateway groups
        SectionList.tsx         # NEW: clickable list of sub-sections under a group
        SectionList.test.tsx    # NEW
    App.test.tsx                # NEW: integration — boot → #runtime/storage → edit → save
```

### State

Extend `AppState`:

```ts
export interface AppState {
  // ...existing fields...
  descriptors: SchemaDescriptor[];     // platform schemas — unchanged
  configSections: ConfigSection[];     // NEW — section schemas loaded at boot
  // ...
}
```

Extend the `boot/loaded` action:

```ts
| { type: 'boot/loaded'; descriptors: SchemaDescriptor[]; configSections: ConfigSection[]; config: Config }
```

Boot in `App.tsx` calls `getSchema()` and `getConfigSchema()` in parallel via `Promise.all`. Both must succeed before dispatching `boot/loaded`; if either fails, dispatch `boot/failed` with the first error message.

Append one action:

```ts
| { type: 'edit/config-field'; sectionKey: string; field: string; value: unknown }
```

Reducer:

```ts
case 'edit/config-field': {
  const prev = (state.config as Record<string, unknown>)[action.sectionKey] ?? {};
  return {
    ...state,
    config: {
      ...state.config,
      [action.sectionKey]: { ...(prev as object), [action.field]: action.value },
    },
  };
}
```

Update `totalDirtyCount`:

```ts
export function totalDirtyCount(state: AppState): number {
  let n = dirtyCount(state);  // gateway platform diffs
  for (const g of GROUPS) {
    if (g.id === 'gateway') continue;
    if (groupDirty(state, g.id)) n++;
  }
  return n;
}
```

One dirty group counts as one change in the TopBar badge. Per-field diffs inside a section are not broken out — matches how Stage 1 counts per-instance, not per-field.

### Shell integration

`shell/sections.ts` registry:

```ts
export interface SectionDef {
  key: string;
  groupId: GroupId;
  plannedStage: string;  // 'done' for storage, 'stage 3' for others
}

export const SECTIONS: SectionDef[] = [
  { key: 'storage', groupId: 'runtime', plannedStage: 'done' },
  // stage 3+ adds more
];

export function sectionsInGroup(g: GroupId): SectionDef[] { /* ... */ }
```

`Sidebar.tsx` renders `<SectionList>` as the `children` of every non-gateway `GroupSection` (Gateway continues to render `GatewaySidebar`). `SectionList` shows one row per section in that group, clickable; clicking dispatches `shell/selectSub` with the section key.

`ContentPanel.tsx` routing grows to:

```ts
if (activeGroup === 'gateway') return <GatewayPanel .../>;
if (activeGroup && activeSubKey) {
  const section = configSections.find(s => s.key === activeSubKey);
  if (section && section.group_id === activeGroup) {
    return <ConfigSection section={section}
                          value={config[section.key] ?? {}}
                          originalValue={originalConfig[section.key] ?? {}}
                          onFieldChange={(field, value) =>
                            dispatch({type: 'edit/config-field', sectionKey: section.key, field, value})} />;
  }
}
return <ComingSoonPanel group={activeGroup} subKey={activeSubKey} .../>;
```

Unknown `subKey` under a known group falls through to `ComingSoonPanel` with a "Coming soon — stage N" label sourced from `sections.ts` (stages 3–7 sections are pre-registered with `plannedStage` so the placeholder label stays accurate).

### Renderer

`ConfigSection.tsx`:

- For each field in `section.fields`, evaluate `visibleWhen` against the current `value`. If the predicate fails, skip rendering (do not hold state, do not emit a hidden input).
- Dispatch by `kind` to the existing field components: `TextInput`, `NumberInput`, `BoolToggle`, `EnumSelect`, `SecretInput`, and the new `FloatInput`.
- For `FieldSecret`, pass `disableReveal={true}` to `SecretInput` (new optional prop added in Stage 2). When set, the Show button is always disabled and its `title` reads "Reveal not supported for this field (stage 2)". When unset (existing platform editor call sites), `SecretInput` behaves exactly as today — the reveal URL it hits (`/api/platforms/{instanceKey}/reveal`) is unchanged.
- No "new / delete / enabled" controls — sections are singletons.

Field components receive the descriptor's `FieldSpec` plus current value + onChange; they already exist from Stage 1's platform editor and accept the extended `SchemaField` shape without changes (we'll widen the TS type union to include the new kind).

## Testing

### Go

- `config/descriptor/descriptor_test.go`:
  - All registered sections have non-empty `Fields`.
  - No field has `Kind == FieldUnknown`.
  - Every `VisibleWhen.Field` references a sibling field name present in the same section.
  - Every `FieldEnum` has non-empty `Enum`.
  - Every `FieldSecret` has a yaml-tagged counterpart in `config.Config` (reflect over the struct; skip on names that can't be resolved to avoid coupling the test to every section's Go type).
- `config/descriptor/storage_test.go`:
  - Storage section's field names match yaml tags on `config.StorageConfig`.
- `api/handlers_config_schema_test.go`:
  - Endpoint returns 200, JSON validates, sections are sorted by `key`.
- `api/handlers_config_test.go`:
  - `GET /api/config` now redacts `storage.postgres_url` to empty string.
  - `PUT /api/config` with `storage.postgres_url = ""` preserves the previous secret (mirror platform behavior).

### Frontend

- `api/schemas.test.ts`: zod happy + sad paths for `ConfigSchemaResponseSchema`.
- `components/ConfigSection.test.tsx`:
  - renders exactly the visible fields.
  - hides fields whose `visible_when` doesn't match current value.
  - flipping the discriminator (`driver: sqlite → postgres`) swaps which field is visible.
  - `onFieldChange` fires with `(name, value)`.
  - `SecretInput`'s Show button is disabled with tooltip.
- `state.test.ts`:
  - `edit/config-field` deep-merges at depth 2.
  - `totalDirtyCount` sums gateway platform diffs + one per dirty non-gateway group.
  - `groupDirty('runtime')` flips when `storage.driver` changes.
- `shell/sections.test.ts`: every SECTION entry references a real group id; `sectionsInGroup` returns sections in a deterministic order.
- `App.test.tsx` (new integration): boot → hash `#runtime/storage` → driver field visible → change to postgres → postgres_url field visible + sqlite_path hidden → TopBar badge shows `Save · 1 changes` → click Save → `PUT /api/config` called with storage slice containing new driver.

## Data Flow

```
┌──────────────┐   GET /api/config/schema    ┌──────────────────────┐
│  web boot    │ ──────────────────────────▶ │ api.handleConfigSchema│
│              │ ◀── {sections: [...]} ───── │  descriptor.All()    │
└──────────────┘                             └──────────────────────┘

┌──────────────┐   GET /api/config            ┌──────────────────────┐
│  web boot    │ ──────────────────────────▶ │ api.handleConfigGet  │
│              │ ◀── {config: {...}} ─────── │ redactSecrets (ext)  │
└──────────────┘                             └──────────────────────┘

user edits driver sqlite → postgres in Storage panel
  └▶ dispatch('edit/config-field', sectionKey: 'storage', field: 'driver', value: 'postgres')
     └▶ state.config.storage.driver = 'postgres'
     └▶ ConfigSection re-evaluates visibleWhen
     └▶ sqlite_path hidden; postgres_url shown
     └▶ groupDirty('runtime') → true
     └▶ totalDirtyCount → +1 → TopBar "Save · 1 changes"

user types postgres://... into postgres_url
  └▶ dispatch('edit/config-field', sectionKey: 'storage', field: 'postgres_url', value: 'postgres://...')

user clicks Save
  └▶ dispatch('save/start')
  └▶ PUT /api/config { config: {...full tree with edited storage...} }
  └▶ server: write to disk; ignore fields where secret came back empty-string
  └▶ 200
  └▶ dispatch('save/done'); originalConfig ← config; badge clears
```

## Error Handling

- `GET /api/config/schema` on startup: if 4xx/5xx, the boot transitions to `'error'` with the server's message. Platform schemas and the rest of the app continue to function only if both the platforms schema and the config schema succeeded — a missing config schema blocks boot, same policy as the existing platforms schema call.
- `edit/config-field` on an unknown section key: no-op (reducer writes into the arbitrary key but `ConfigSection` never dispatches an unknown key since it's driven by the schema).
- A descriptor with a broken `VisibleWhen` reference (field name that doesn't exist in the section): caught by `descriptor_test.go` at build time. In prod, the renderer treats a missing referent as "predicate fails" and hides the field; the test gate means this path shouldn't fire in practice.
- `FieldSecret` input left empty on save: server preserves the previous value (existing behavior for platform secrets, extended here).
- Float input: `FloatInput` is a thin copy of `NumberInput` with `type="number"` + `step="any"`. Like `NumberInput`, it passes the raw string value through `onChange`; parsing to a concrete float happens at serialization time (the YAML marshaller on the server). Browser-native validation shows invalid input; no additional error UI in Stage 2.

## Rollback

Every addition is additive. A full revert removes:
- `config/descriptor/` package
- `api/handlers_config_schema.go` + its route registration
- The redactSecrets loop extension in `api/handlers_config.go`
- `web/src/components/ConfigSection.tsx`, `FloatInput.tsx`, `SectionList.tsx` + tests
- `web/src/shell/sections.ts` + tests
- `edit/config-field` action and `totalDirtyCount` extension in `state.ts`
- `App.tsx` / `ContentPanel.tsx` / `Sidebar.tsx` / `App.test.tsx` diffs
- `api/schemas.ts` additions

After revert the shell reverts to Stage-1 behavior exactly: Gateway works; other groups show `ComingSoonPanel`; no Storage editor.

## Completion Gate

Before calling Stage 2 done:
- `go test ./config/descriptor/... ./api/...` passes.
- `cd web && pnpm test` passes (vitest; includes the new integration test).
- `cd web && pnpm type-check && pnpm lint` passes with zero warnings.
- `make web-check` passes (includes the `api/webroot/` sync assertion; the rebuilt bundle embeds the new endpoint response).
- Manual smoke: `hermind web` → navigate to `#runtime/storage` → flip driver → observe field swap → save → grep `storage:` block in the written YAML file and confirm `driver: postgres` and `postgres_url: ...` present, `sqlite_path` absent or empty.
- `docs/smoke/web-config.md` gains a `## Stage 2 · Schema infrastructure (Storage)` section mirroring the above steps.

Once all boxes are checked, Stage 2 is complete. Stage 3 (simple sections) can begin by adding descriptors for Logging / Metrics / Tracing / Agent / Terminal / Model and registering them in the sections registry — no further infrastructure work required.
