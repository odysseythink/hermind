# Web Config Schema Infrastructure — Stage 4b Design

**Goal:** Ship the Providers editor — a map-of-uniform-structs section under the Models group — plus the fetch-models endpoint that doubles as a connection test.

**Sub-stage context:** Stage 4 as a whole lights up the Models group. 4a shipped the Default Model scalar + Auxiliary single-struct. 4b ships the keyed-map editor (the core of the Models group). 4c will ship the fallback-providers list. The three sub-stages share the Stage 4a scalar-section infrastructure but each adds its own shape discriminant.

**Non-goals for 4b:**
- Fallback providers list (4c).
- Cross-section datalist wiring: the Default Model field in 4a's `model` scalar section and the `model` field in 4a's `auxiliary` section will NOT pick up a fetch-models datalist in 4b. Autocomplete here is scoped to the per-provider-instance Model field that lives inside the providers editor itself. Cross-section datalists can land later as a `datalist_source` descriptor field once a real second consumer exists.
- Connection test separate from fetch-models. Fetch-models IS the connection test — success returns model IDs, failure returns an error. One endpoint, one button.
- Preview-before-save flow. The fetch-models endpoint reads stored config; users must Save before testing. Matches the existing `POST /api/platforms/{key}/test` contract.

---

## Architecture

Stage 4a established that descriptors discriminate on `Section.Shape`:

- `ShapeMap` — value is `map[string]any` (default — every Stage 2/3 section, plus `auxiliary`).
- `ShapeScalar` — value is a raw scalar (`model`).

Stage 4b adds:

- `ShapeKeyedMap` — value is `map[string]map[string]any`. Each instance has the same `Fields []FieldSpec` schema. The map key is an arbitrary user-supplied identifier (e.g. `anthropic_main`); the map value is a Record whose keys match the descriptor's `Fields` names.

Every non-gateway keyed-map section in the future (cron jobs, MCP servers, …) can register this shape. For 4b we register exactly one: `providers`. Gateway platforms stays on its own per-type descriptor system because each platform type has distinct fields — a uniform-schema pattern doesn't fit.

Four backend touchpoints branch on the new shape:

1. `redactSectionSecrets` — iterates `m[sec.Key].(map[string]any)`, and for each instance (also `map[string]any`) blanks every `FieldSecret` field.
2. `preserveSectionSecrets` — same shape, restores blanks from `current` config on empty PUT.
3. `handleConfigSchema` — emits `"shape": "keyed_map"` in the DTO when `sec.Shape == ShapeKeyedMap`.
4. `TestSectionInvariants` — enforces `ShapeKeyedMap` sections have `len(Fields) > 0` AND include exactly one `FieldEnum` named `provider` (used by the `NewProviderDialog` as the type picker and self-describing the enum source).

One new API endpoint: `POST /api/providers/{name}/models`. Reads the stored `config.Providers[name]`, dispatches via `factory.New`, type-asserts `provider.ModelLister`, calls `ListModels` with a 10s timeout. Returns `{models: ["<id>", ...]}` on success, HTTP 4xx/5xx with a human-readable body on failure. Ports the legacy `cli/ui/webconfig/handlers.go:258` into the Stage-1+ `api/` package.

The frontend gets three new components scoped to `web/src/components/groups/models/`:
- `ModelsSidebar` — instance list + "+ New provider" button under the Models group (mirrors `GatewaySidebar`).
- `ProviderEditor` — main-pane editor for one instance; wraps the generic `<ConfigSection>` around that instance's 4-field value, adds a header with Delete and a footer with Fetch-models.
- `NewProviderDialog` — cloned from `NewInstanceDialog` (same key-regex validation, same shape-and-button UX), scoped to the providers descriptor's `provider` enum.

Three new state actions land under the generic `keyed-instance/*` namespace (not `provider/*`) so future `ShapeKeyedMap` sections can reuse them without rename churn.

---

## Backend components

### §A Descriptor infrastructure (`config/descriptor/descriptor.go`)

```go
const (
    ShapeMap      SectionShape = iota
    ShapeScalar
    ShapeKeyedMap
)
```

`TestSectionInvariants` gains one block:

```go
if s.Shape == ShapeKeyedMap {
    if len(s.Fields) == 0 {
        t.Errorf("section %q: ShapeKeyedMap requires at least 1 field", s.Key)
    }
    var providerEnums int
    for _, f := range s.Fields {
        if f.Name == "provider" && f.Kind == FieldEnum {
            providerEnums++
        }
    }
    if providerEnums != 1 {
        t.Errorf("section %q: ShapeKeyedMap requires exactly one FieldEnum named \"provider\" (got %d)",
            s.Key, providerEnums)
    }
}
```

The "provider enum" invariant encodes the self-description the frontend relies on — `NewProviderDialog` reads the descriptor's `provider` enum to populate the type dropdown without having to know `factory.Types()` directly.

### §B Providers descriptor (`config/descriptor/providers.go`)

```go
package descriptor

import "github.com/odysseythink/hermind/provider/factory"

func init() {
    Register(Section{
        Key:     "providers",
        Label:   "Providers",
        Summary: "LLM providers available to Default Model, Auxiliary, and fallback.",
        GroupID: "models",
        Shape:   ShapeKeyedMap,
        Fields: []FieldSpec{
            {
                Name:     "provider",
                Label:    "Provider type",
                Kind:     FieldEnum,
                Required: true,
                Enum:     factory.Types(),
            },
            {Name: "base_url", Label: "Base URL", Kind: FieldString},
            {
                Name:     "api_key",
                Label:    "API key",
                Kind:     FieldSecret,
                Required: true,
            },
            {
                Name:  "model",
                Label: "Default model for this provider",
                Help:  "Optional — provider-qualified id used when a request doesn't pin a specific model.",
                Kind:  FieldString,
            },
        },
    })
}
```

Field names match the YAML tags on `config.ProviderConfig` exactly: `provider`, `base_url`, `api_key`, `model`. This is load-bearing for `preserveSectionSecrets`'s YAML round-trip.

### §C Redact / preserve branches (`api/handlers_config.go`)

Both functions already walk `descriptor.All()` with `Shape == ShapeScalar` early-continues added in Stage 4a. 4b extends the loop with a second branch before the existing map-assumption code:

```go
for _, sec := range descriptor.All() {
    if sec.Shape == descriptor.ShapeScalar {
        continue
    }
    if sec.Shape == descriptor.ShapeKeyedMap {
        // m[sec.Key] is map[string]any where each entry is itself map[string]any.
        outer, ok := m[sec.Key].(map[string]any)
        if !ok { continue }
        for _, instance := range outer {
            inner, ok := instance.(map[string]any)
            if !ok { continue }
            for _, f := range sec.Fields {
                if f.Kind != descriptor.FieldSecret { continue }
                if _, present := inner[f.Name]; present {
                    inner[f.Name] = ""
                }
            }
        }
        continue
    }
    // existing ShapeMap path
    blob, ok := m[sec.Key].(map[string]any)
    // …
}
```

`preserveSectionSecrets` mirrors this: iterate outer map, match instances between `updM` and `curM` by key, restore blanks per instance. Keys missing from `curM` (newly-created instances) are left as-is — same behavior as `preservePlatformSecrets` for a new gateway platform.

### §D Schema DTO (`api/handlers_config_schema.go`)

The `shapeString` helper added in Stage 4a already returns `""` for `ShapeMap` (omits key) and `"scalar"` for `ShapeScalar`. Extend it:

```go
func shapeString(s descriptor.SectionShape) string {
    switch s {
    case descriptor.ShapeScalar:
        return "scalar"
    case descriptor.ShapeKeyedMap:
        return "keyed_map"
    default:
        return ""
    }
}
```

### §E Fetch-models endpoint (`api/handlers_providers_models.go`)

New file, new handler:

```go
// POST /api/providers/{name}/models
func (s *Server) handleProvidersModels(w http.ResponseWriter, r *http.Request) {
    name := chi.URLParam(r, "name")
    cfg, ok := s.opts.Config.Providers[name]
    if !ok {
        http.Error(w, "unknown provider", http.StatusNotFound)
        return
    }
    p, err := factory.New(cfg)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    lister, ok := p.(provider.ModelLister)
    if !ok {
        http.Error(w,
            fmt.Sprintf("provider %q does not support model listing", cfg.Provider),
            http.StatusNotImplemented)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()
    models, err := lister.ListModels(ctx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }
    writeJSON(w, struct {
        Models []string `json:"models"`
    }{Models: models})
}
```

Route wiring in `api/server.go`, mounted under the existing Bearer-auth middleware:

```go
r.Post("/api/providers/{name}/models", s.handleProvidersModels)
```

Status code summary:
- 200 — `{"models": [...]}`
- 400 — factory.New rejected the config (malformed `base_url`, missing required field, etc.)
- 404 — no `Providers[name]` in stored config
- 501 — provider type exists but its constructor returns something that doesn't implement `ModelLister`
- 502 — upstream provider errored (network, auth, rate-limit, etc.)

Legacy endpoint at `cli/ui/webconfig/handlers.go:258` is not touched by this work. It lives on its own server + doc layer and will be retired when the legacy webconfig package is removed (separate cleanup, out of 4b scope).

---

## Frontend components

### §F Zod schema + state (`web/src/api/schemas.ts`, `web/src/state.ts`)

Schema extension — add the new shape variant and the fetch-models response:

```ts
export const ConfigSectionSchema = z.object({
    key: z.string(),
    label: z.string(),
    summary: z.string().optional(),
    group_id: z.string(),
    shape: z.enum(['map', 'scalar', 'keyed_map']).optional(),
    fields: z.array(ConfigFieldSchema),
});

export const ProviderModelsResponseSchema = z.object({
    models: z.array(z.string()),
});
export type ProviderModelsResponse = z.infer<typeof ProviderModelsResponseSchema>;
```

State actions — three new variants in the `Action` union:

```ts
| { type: 'edit/keyed-instance-field'; sectionKey: string; instanceKey: string; field: string; value: unknown }
| { type: 'keyed-instance/create';     sectionKey: string; instanceKey: string; initial: Record<string, unknown> }
| { type: 'keyed-instance/delete';     sectionKey: string; instanceKey: string }
```

Reducer cases:

- **`edit/keyed-instance-field`** — deep-merge `{[sectionKey]: {[instanceKey]: {[field]: value}}}` into `state.config`, preserving other instances and other fields.
- **`keyed-instance/create`** — spreads `{[instanceKey]: initial}` into `state.config[sectionKey]`. If the section doesn't exist in config yet (empty providers map), create it.
- **`keyed-instance/delete`** — removes `instanceKey` from `state.config[sectionKey]`.

Dirty tracking piggybacks on the existing `totalDirtyCount` which diffs `config` against `originalConfig`. No special-case counter.

### §G `ContentPanel` + `Sidebar` branches

**`ContentPanel.tsx`** — after the `scalar` branch, add a `keyed_map` branch:

```tsx
if (section.shape === 'keyed_map') {
    if (!props.activeSubKey) {
        return <EmptyState onSelectGroup={props.onSelectGroup} />; // "pick an instance"
    }
    return (
        <ProviderEditor
            sectionKey={section.key}
            instanceKey={props.activeSubKey}
            section={section}
            config={props.config}
            originalConfig={props.originalConfig}
            onField={/* dispatches edit/keyed-instance-field */}
            onDelete={/* dispatches keyed-instance/delete */}
        />
    );
}
```

**`Sidebar.tsx`** — replace the "render `SectionList` for everything non-gateway" branch with a key check:

```tsx
// inside the GROUPS.map loop:
const body = g.id === 'gateway' ? <GatewaySidebar ... />
           : g.id === 'models'  ? <ProvidersSidebar
                                     instances={providerInstances}
                                     activeSubKey={props.activeSubKey}
                                     dirtyKeys={dirtyProviderKeys}
                                     onSelect={props.onSelectSub}
                                     onNewInstance={props.onNewProviderDialog}
                                   />
           :                        <SectionList ... />;
```

`ModelsSidebar` shows the scalar `model` entry at the top (unchanged) followed by the keyed provider instance list. `SectionList` keeps handling observability + runtime exactly as it does today.

**State wiring** — `App.tsx` computes:

```ts
const providerInstances = useMemo(() => {
    const p = (state.config.providers as Record<string, Record<string, unknown>>) ?? {};
    return Object.keys(p).sort().map(k => ({
        key: k,
        type: (p[k].provider as string) ?? '',
    }));
}, [state.config.providers]);
```

…and passes it through Sidebar props.

### §H Three new components under `web/src/components/groups/models/`

**`ModelsSidebar.tsx`** — renders the instance list:
- Each row: `{instanceKey}` in normal weight + `{providerType}` badge in muted.
- Active row highlighted via `aria-current="true"`.
- Dirty dot per instance (reads `originalConfig.providers[key]` vs `config.providers[key]` via a `keyedInstanceDirty` selector modeled on the existing `instanceDirty`).
- "+ New provider" button at bottom, opens `NewProviderDialog`.

**`ProviderEditor.tsx`** — main-pane editor for a selected instance:
- **Header:** breadcrumb "Models / Providers / {key}" + Delete button (confirm via `window.confirm` — same pattern gateway uses).
- **Body:** `<ConfigSection>` rendered once with the instance's 4-field slice as `value`. The descriptor comes from `configSections.find(s => s.key === 'providers')`. The Model field's `<input>` carries a `list` attribute pointing at a `<datalist>` populated from component-local state `fetchedModels: string[]`.
- **Footer action bar:**
    - `Fetch models` button — disabled when the instance slice is dirty (tooltip "Save first, then fetch models"). On click, `POST /api/providers/{instanceKey}/models`. On success: status chip "Connected ✓ ({N} models)" + hydrate the datalist. On failure: status chip with the HTTP error body, a red dot, no datalist update. The 10s server timeout is mirrored by an `AbortController` on the frontend.

**`NewProviderDialog.tsx`** — cloned from `NewInstanceDialog.tsx`:
- Key input: regex `^[a-z0-9_]+$`, red inline error if malformed.
- Type dropdown: options sourced from the providers descriptor's `provider` FieldSpec `enum` (NOT `factory.Types()` directly — the descriptor is the single source of truth).
- Create button: dispatches `keyed-instance/create` with `{sectionKey: 'providers', instanceKey: <key>, initial: {provider: <type>, base_url: '', api_key: '', model: ''}}`, then `shell/selectGroup('models')` + `shell/selectSub(<key>)` + closes the dialog.
- Cancel button: closes dialog without side-effect.

### §I `sections.ts` registration

Append `providers` after `model` in the models group:

```ts
{ key: 'model',     groupId: 'models', plannedStage: 'done' },
{ key: 'providers', groupId: 'models', plannedStage: 'done' }, // new
```

Sidebar declaration order for the models group becomes `['model', 'providers']`.

---

## Data flow

### Reading providers

1. Boot `GET /api/config/schema` returns `providers` with `shape: "keyed_map"` and the 4-field schema. Stored in `state.configSections`.
2. Boot `GET /api/config` returns `"providers": {"anthropic_main": {"provider": "anthropic", "base_url": "", "api_key": "", "model": ""}}` with `api_key` blanked by `redactSectionSecrets`. Stored in `state.config.providers`.
3. `App.tsx` derives `providerInstances` from the map, passes through Sidebar → `ModelsSidebar`.
4. Navigating to `#models/anthropic_main` → ContentPanel's `keyed_map` branch renders `<ProviderEditor instanceKey="anthropic_main" />`.

### Editing & saving

1. User edits the `base_url` field: `<TextInput>` fires onChange → `<ConfigSection>` calls `onFieldChange('base_url', 'https://…')` → `ProviderEditor` dispatches `edit/keyed-instance-field` with the instance key. Reducer deep-merges into `state.config.providers.anthropic_main.base_url`.
2. Existing top-bar dirty counter sees a diff against `originalConfig` → "Save · 1 change".
3. User clicks Save → existing save flow sends the full `state.config` as JSON → backend `yaml.Unmarshal` writes the field → `preserveSectionSecrets` restores any blank api_keys from stored config → `SaveToPath` flushes YAML to disk.

### Creating & deleting instances

- Create: NewProviderDialog dispatches `keyed-instance/create` with `{provider: 'anthropic', base_url: '', api_key: '', model: ''}`. Route jumps to the new editor. The instance is dirty until saved.
- Delete: ProviderEditor dispatches `keyed-instance/delete`, clears `activeSubKey`, falls back to the Models group's empty state. Dirty until saved (the deletion is a diff against originalConfig).

### Fetch models

1. User clicks Fetch models. If dirty, button is disabled (tooltip "Save first, then fetch models").
2. `POST /api/providers/anthropic_main/models` with Bearer auth. 10s timeout on the frontend (AbortController) to match the backend's `context.WithTimeout`.
3. 200 → component-local state `{status: 'ok', models: [...]}`. Status chip renders "Connected ✓ (12 models)". The Model field's `<input list="…">` references a `<datalist>` populated with the returned strings.
4. 4xx/5xx → `{status: 'err', error: body}`. Red chip with the error text.

---

## Error handling

- **`ShapeKeyedMap` section with len(Fields)==0 or no provider-enum** → `TestSectionInvariants` fails at test time. Registration still succeeds; the invariant is the gate.
- **Fetch-models on a non-ModelLister provider** → 501, red chip "provider 'x' does not support model listing". The button stays enabled so the user can inspect other instances.
- **Fetch-models upstream error** → 502, red chip with the underlying error (rate limit, auth rejection, DNS failure, etc). User fixes the api_key or base_url and saves, then tries again.
- **Fetch-models network timeout** → frontend AbortController fires at 10s; renders "timeout — check base_url / network".
- **Creating a provider with an existing key** → `NewProviderDialog` checks `existingKeys: Set<string>` from config.providers keys and disables Create with the red error "key already exists" (same as gateway).
- **Deleting the currently-selected provider** → reducer's `keyed-instance/delete` clears `shell.activeSubKey` if it matches the deleted key. ContentPanel falls back to the Models group's EmptyState.
- **Secret round-trip edge case** — user creates `a`, types an api_key, saves; then edits `a`, clears api_key, saves. Expected: stored api_key is RESTORED by `preserveSectionSecrets` (can't clear via the web UI, same as storage.postgres_url and auxiliary.api_key). To truly delete a stored key, edit `config.yaml`. This is documented-behavior, not a bug; the smoke doc says so.

---

## Testing

### Go
- `config/descriptor/providers_test.go` — registration, Shape=ShapeKeyedMap, 4 fields of expected kinds including FieldSecret api_key, provider enum includes "anthropic" and "openai".
- `config/descriptor/descriptor_test.go` — extend TestSectionInvariants + add inline-seeded `TestShapeKeyedMapInvariant_RequiresProviderEnum` test proving the invariant fires.
- `api/handlers_providers_models_test.go` — 5 cases: happy path (200 + models), unknown name (404), factory error (400), non-ModelLister (501), upstream error (502). Uses a fake `provider.Provider` fixture + registers a fake `ModelLister` for the 200 path.
- `api/handlers_config_schema_test.go` — new `TestConfigSchema_IncludesStage4bSections` pinning `providers.shape=="keyed_map"`, `providers.group_id=="models"`, exactly one `FieldEnum` named `provider`, `api_key.kind=="secret"`.
- `api/handlers_config_test.go` — GET with stored `providers.anthropic_main.api_key="sk-real"` returns it blanked; PUT with an empty api_key restores it; PUT with a different non-empty api_key overwrites (new key persists).

### Web
- `web/src/api/schemas.test.ts` — parses `shape: "keyed_map"`; ProviderModelsResponseSchema accepts `{models: ["m1"]}` and rejects malformed inputs.
- `web/src/state.test.ts` — three new describe blocks, one per reducer case. Each asserts the correct shape of `state.config` after the dispatch, including no-op paths (delete of non-existent key, create over existing key, edit on an instance that doesn't exist yet).
- `web/src/shell/sections.test.ts` — models group order is `['model', 'providers']`; `providers` plannedStage='done'.
- `web/src/components/groups/models/ProvidersSidebar.test.tsx` — renders instances, click dispatches selectSub, "+ New" fires the new-instance prop, dirty dot appears when config diverges.
- `web/src/components/groups/models/ProviderEditor.test.tsx` — renders 4 fields, field edits dispatch the right action, Fetch-models button disabled when dirty, successful fetch populates a datalist, failed fetch renders the error chip, Delete prompts confirm + dispatches delete.
- `web/src/components/groups/models/NewProviderDialog.test.tsx` — regex validation fires on bad keys, type dropdown options match the descriptor's provider enum, Create dispatches the right action payload, Cancel leaves state untouched.

### Smoke doc

Append `## Stage 4b · Providers + fetch-models` to `docs/smoke/web-config.md`:
- Navigate to Models → click "+ New provider". Modal opens. Try key `BAD KEY` → red error. Change to `anthropic_main`, pick type anthropic, click Create. Routes to `#models/anthropic_main`.
- Fill base_url, api_key (real key), model. Save. Click Fetch models → green chip "Connected ✓ ({N} models)" + typing in the Model field shows autocomplete suggestions from the returned list.
- Flip api_key to garbage, Save, click Fetch models → red chip with the real upstream error.
- Re-visit `#models/anthropic_main`. API key field is blanked (redact on GET). Clear the field and Save → the stored api_key is preserved (same contract as `storage.postgres_url`).
- Click Delete, confirm → sidebar row disappears, content pane returns to the Models group empty state.
- Grep `~/.hermind/config.yaml` before/after: `providers: {}` initially; after Save `providers: { anthropic_main: { provider: anthropic, base_url: ..., api_key: ..., model: ... } }`. After Delete + Save: `providers: {}` again (or absent if yaml/omitempty drops it).

---

## Delivery plan

Ten tasks, each self-contained and test-first:

1. **`ShapeKeyedMap` infra.** Add the constant, extend TestSectionInvariants, add the inline-seeded invariant test.
2. **Backend redact/preserve for ShapeKeyedMap.** Second branch in both functions, iterates instances.
3. **Schema DTO emits `"keyed_map"`.** Extend `shapeString`; extend the Stage 4a contract test to cover the new shape string.
4. **`providers` descriptor + test.** Single init() calling Register with the 4 fields; test locks the contract.
5. **`POST /api/providers/{name}/models` handler + route wiring + 5-case test.**
6. **Frontend Zod schema extension + `ProviderModelsResponseSchema` + test.**
7. **State: three `keyed-instance/*` actions + reducers + tests.**
8. **`ModelsSidebar` + `ProviderEditor` + `NewProviderDialog` components + tests. `ContentPanel` + `Sidebar` branches.**
9. **`sections.ts` registers `providers` with plannedStage='done'; declaration-order test updated.**
10. **Gauntlet + webroot rebuild + smoke doc append.**

Cross-task dependencies:
- 4 → 1 (descriptor uses ShapeKeyedMap).
- 5 → 4 (endpoint's integration test references real provider instances via the descriptor).
- 2 → 1 (redact branch references the constant).
- 3 → 1 (schema emission).
- 8 → 6, 7 (components consume Zod types and dispatch new actions).
- 10 runs last and touches only webroot + docs.

No deep chain; tasks 1-7 are mostly independent after Task 1 lands.

---

## What this does NOT change

- Stage 2/3/4a descriptors (storage, agent, terminal, auxiliary, logging, metrics, tracing, model). Every existing `ShapeMap` or `ShapeScalar` section stays byte-identical on the wire.
- Gateway infrastructure. `gateway/platforms` remains the home of per-type platform descriptors; providers does NOT migrate there.
- Legacy `cli/ui/webconfig/handlers.go:258`. It continues to serve its own web UI until the legacy webconfig package is deleted in a separate cleanup.
- Default Model and Auxiliary Model autocomplete. Both continue to render as plain text inputs. Cross-section datalist wiring is deferred until a `datalist_source` FieldSpec field is justified by multiple consumers.

---

## Open questions carried into 4c

- **Fallback providers (list-valued).** 4c introduces `ShapeList` and a `fallback_providers` descriptor. The design question: does a list of ProviderConfig share the UI shell with ShapeKeyedMap (reordering + per-entry editor)? Probably yes — both are collection types. Factor the shared keyboard / drag reorder machinery out when 4c lands.
- **Cross-section datalist.** Once 4b's fetch-models is proven, a future change could add a `datalist_source: {section: "providers", instance_field: "provider", model_field: "model"}` hint to the scalar Model field so the Default Model autocomplete pulls from any configured provider.
- **Connection-test as capability.** Today the Fetch-models button is providers-specific UI. If a future `ShapeKeyedMap` section needs a different test action (e.g. cron.jobs "run once now"), factor an `actions: [{id, label, endpoint}]` DTO field or a per-section component map. YAGNI until the second consumer appears.
