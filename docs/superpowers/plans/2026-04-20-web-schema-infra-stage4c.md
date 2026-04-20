# Web Config Schema Infrastructure (Stage 4c) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Fallback Providers editor — a `[]ProviderConfig` ordered-list section under the Models group — so users can configure the fallback chain consumed by `provider.FallbackChain` from the web UI.

**Architecture:** Stage 4a introduced `Section.Shape` with `ShapeScalar`. Stage 4b added `ShapeKeyedMap` for map-of-uniform-struct sections. Stage 4c adds `ShapeList` for ordered-list-of-uniform-struct sections. Every `ShapeList` element conforms to the same `Fields` schema; each element's secret fields round-trip through redact/preserve, addressed by **index** (not stable ID). Reorder is explicit up/down buttons — drag-and-drop is deferred. `ProviderConfig` is the same 4-field struct as Stage 4b; we reuse `ConfigSection` for the field renderer and introduce a thin `FallbackProviderEditor` shell for the delete/up/down chrome and position indicator.

**Tech Stack:** Go 1.21+ (config/descriptor, api), TypeScript + React + Zod (web/src), Vitest, Go `testing`, react-testing-library.

**Spec:** Reuse `docs/superpowers/specs/2026-04-20-web-schema-infra-stage4b-design.md` §"Open questions carried into 4c" for the architectural context. No separate 4c design doc — this plan is self-contained.

**Scope note — what this does NOT ship:**
- **No fetch-models button for fallback entries.** Per-provider connection testing is providers-section-only in Stage 4b; fallback users can validate a provider type by configuring it as a primary provider first. If fallback-specific fetch-models is needed, it's 4d.
- **No drag-and-drop reorder.** Up/Down buttons move one slot; first item's Up is disabled, last item's Down is disabled.
- **No separate "NewFallbackDialog".** A `+ Add fallback` button appends a blank entry at the end with the first provider-type enum value pre-selected; user edits the row inline.
- **No stable IDs for list elements.** Preserve is strictly by index. Reordering or deleting a row with a stored secret invalidates the secret-carry — user must re-enter the API key. This is documented in the smoke doc as "after deleting or reordering fallback providers, re-enter API keys for affected rows before saving."
- **No list validation beyond what each field already enforces.** Empty list is allowed (zero fallbacks configured). Duplicate provider types are allowed (a user might want two `anthropic` entries pointing at different regions).
- **No cross-section datalist** (same scope-out as 4b).

---

## File Structure

**Created (Go):**
- `config/descriptor/fallback_providers.go` — `ShapeList` section registering the 4 ProviderConfig fields (identical field schema to `providers.go`).
- `config/descriptor/fallback_providers_test.go` — field-kind assertions + provider-enum floor + shape assertion.

**Modified (Go):**
- `config/descriptor/descriptor.go` — add `ShapeList` const and document it on `SectionShape`.
- `config/descriptor/descriptor_test.go` — invariant extension + distinctness test for `ShapeList`.
- `api/handlers_config_schema.go` — `shapeString` returns `"list"` for `ShapeList`.
- `api/handlers_config.go` — add `ShapeList` branch in `redactSectionSecrets` (walks `[]any`) and in `preserveSectionSecrets` (walks by index; preserves secret only when updated[i] is blank AND current[i] exists).
- `api/handlers_config_schema_test.go` — new `TestConfigSchema_IncludesStage4cSections` pinning the `fallback_providers` section + shape.
- `api/handlers_config_test.go` — new `TestConfigGet_RedactsListSecrets` + `TestConfigPut_PreservesListSecretsByIndex` (round-trip checks for a list element).
- `config/descriptor/descriptor.go` — (already covered above; listed once).

**Created (TypeScript):**
- `web/src/components/groups/models/FallbackProviderEditor.tsx` — main-pane editor for one fallback entry (4 fields + Delete + position header).
- `web/src/components/groups/models/FallbackProviderEditor.module.css`.
- `web/src/components/groups/models/FallbackProviderEditor.test.tsx`.
- `web/src/shell/listInstances.ts` — `listInstanceDirty(state, sectionKey, index)` selector used by ModelsSidebar for the per-row dirty dot.
- `web/src/shell/listInstances.test.ts`.

**Modified (TypeScript):**
- `web/src/api/schemas.ts` — `shape` enum adds `'list'`.
- `web/src/api/schemas.test.ts` — parse/reject cases for `'list'`.
- `web/src/state.ts` — five new `list-instance/*` actions + reducer cases (create, delete, edit-field, move-up, move-down).
- `web/src/state.test.ts` — five new describe blocks for the reducer cases.
- `web/src/shell/sections.ts` — append `{ key: 'fallback_providers', groupId: 'models', plannedStage: 'done' }`.
- `web/src/shell/sections.test.ts` — models group order is `['model', 'providers', 'fallback_providers']`.
- `web/src/components/shell/ContentPanel.tsx` — `list` branch routes to `FallbackProviderEditor` when `activeSubKey` is a synthetic `fallback:N` key. New props `onConfigListField`, `onConfigListDelete`, `onConfigListMove`.
- `web/src/components/shell/ContentPanel.test.tsx` — new describe block for the `list` routing.
- `web/src/components/groups/models/ModelsSidebar.tsx` — render a "Fallback Providers" section below Providers with list rows and a "+ Add fallback" button. Each row shows "#N <provider-type>" plus up/down/dirty chrome.
- `web/src/components/groups/models/ModelsSidebar.test.tsx` — new describe block covering the fallback rows + Add button + up/down button disabled states.
- `web/src/components/groups/models/ModelsSidebar.module.css` — styles for the fallback rows, position badges, up/down buttons.
- `web/src/components/shell/Sidebar.tsx` — extend `SidebarProps` with `fallbackProviders: Array<{ provider: string }>`, `dirtyFallbackIndices: Set<number>`, `onAddFallback: () => void`, `onMoveFallback: (from: number, dir: 'up' | 'down') => void`. Pass through to ModelsSidebar.
- `web/src/components/shell/Sidebar.test.tsx` — extend `baseProps` defaults + add a fallback-row rendering assertion to the models-group describe.
- `web/src/App.tsx` — compute `fallbackProviders` + `dirtyFallbackIndices` memos, wire `onAddFallback`, `onMoveFallback`, `onConfigListField`, `onConfigListDelete` dispatchers. Synthetic hash key scheme: `#models/fallback:N`.
- `docs/smoke/web-config.md` — append `## Stage 4c · Fallback Providers editor`.

**Rebuilt (generated):**
- `api/webroot/` — regenerated by `make web-check`.

---

## Task 1: `ShapeList` constant + invariant

**Files:**
- Modify: `config/descriptor/descriptor.go`
- Modify: `config/descriptor/descriptor_test.go`

- [ ] **Step 1: Write the failing tests**

Append to the end of `config/descriptor/descriptor_test.go`:

```go
func TestSectionShape_ListConstantDistinct(t *testing.T) {
	// ShapeList must be distinct from ShapeMap (zero value), ShapeScalar
	// (Stage 4a), and ShapeKeyedMap (Stage 4b). A collision would silently
	// break the schema DTO's shape-string emission.
	if ShapeList == ShapeMap {
		t.Error("ShapeList equals ShapeMap — they must be distinct")
	}
	if ShapeList == ShapeScalar {
		t.Error("ShapeList equals ShapeScalar — they must be distinct")
	}
	if ShapeList == ShapeKeyedMap {
		t.Error("ShapeList equals ShapeKeyedMap — they must be distinct")
	}
}

func TestShapeListInvariant_RequiresProviderEnum(t *testing.T) {
	// Seed a ShapeList section without a provider-type discriminator and
	// verify the invariants logic flags it. fallback_providers mandates a
	// provider enum the same way providers (4b) does.
	key := "__test_list_no_provider"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test List",
		GroupID: "runtime",
		Shape:   ShapeList,
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Base URL", Kind: FieldString},
		},
	})
	got := AssertInvariants()
	if len(got) == 0 {
		t.Error("expected invariant failure for ShapeList section without provider enum, got none")
	}
}
```

If `AssertInvariants` doesn't yet exist (Stage 4b may have used a different name), check `descriptor_test.go` for the prevailing invariants-check helper and mirror the call. The test must fail today — if it passes, the invariants aren't checking ShapeList yet and this task must still add the check.

- [ ] **Step 2: Run tests to verify they fail**

```
cd /path/to/hermind && go test ./config/descriptor/...
```

Expected: `TestSectionShape_ListConstantDistinct` fails to compile (undefined ShapeList). `TestShapeListInvariant_RequiresProviderEnum` also fails to compile for the same reason. Both compilation failures satisfy the failing-test gate.

- [ ] **Step 3: Write minimal implementation**

Edit `config/descriptor/descriptor.go`. Locate the `SectionShape` const block and extend it:

```go
const (
	ShapeMap SectionShape = iota
	ShapeScalar
	ShapeKeyedMap
	ShapeList
)
```

Update the godoc above `SectionShape` to append a fourth bullet:

```go
//   - ShapeList     — value is []map[string]any (ordered list of uniform
//                     struct elements, e.g. fallback_providers). Fields
//                     describe one element; exactly one FieldEnum named
//                     "provider" must serve as the type discriminator.
//                     Preservation of secret fields is strictly by index.
```

If `AssertInvariants` already walks `ShapeKeyedMap` and checks for the `provider` enum, extend the same walk to include `ShapeList`. If the invariants are seeded per-shape in a table, add a `ShapeList` entry that requires the provider enum. Match the existing code style — do not refactor.

- [ ] **Step 4: Run tests to verify they pass**

```
cd /path/to/hermind && go test ./config/descriptor/...
```

Expected: PASS (existing Stage 3 + 4a + 4b tests plus the two new ones). Any other failure — stop and investigate.

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/descriptor.go config/descriptor/descriptor_test.go
git commit -m "feat(config/descriptor): ShapeList for ordered-list-of-uniform-struct sections"
```

---

## Task 2: Schema DTO emits `"list"`

**Files:**
- Modify: `api/handlers_config_schema.go`
- Modify: `api/handlers_config_schema_test.go`

Depends on Task 1 (ShapeList constant).

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_config_schema_test.go`:

```go
func TestConfigSchema_EmitsListShapeString(t *testing.T) {
	// A ShapeList section must serialize with shape: "list". Without the
	// string, the frontend Zod schema would reject the response.
	key := "__test_list_shape"
	defer descriptor.Unregister(key) // or delete(registry, key) via a test helper
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test List",
		GroupID: "runtime",
		Shape:   descriptor.ShapeList,
		Fields: []descriptor.FieldSpec{
			{
				Name: "provider", Label: "Provider type",
				Kind: descriptor.FieldEnum, Required: true,
				Enum: []string{"anthropic"},
			},
		},
	})

	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.handleConfigSchema(rr, httptest.NewRequest(http.MethodGet, "/api/config/schema", nil))

	var resp ConfigSchemaResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := false
	for _, sec := range resp.Sections {
		if sec.Key == key {
			found = true
			if sec.Shape != "list" {
				t.Errorf("shape = %q, want %q", sec.Shape, "list")
			}
		}
	}
	if !found {
		t.Fatalf("section %q not emitted", key)
	}
}
```

If `descriptor.Unregister` doesn't exist, either add it (tiny one-liner wrapping `delete(registry, key)`) or reach in via the test-only helper used by Stage 4b's tests. Check how Stage 4b tests clean up — mirror exactly.

- [ ] **Step 2: Run test to verify it fails**

```
cd /path/to/hermind && go test ./api/... -run TestConfigSchema_EmitsListShapeString
```

Expected: FAIL. The current `shapeString` returns `""` for `ShapeList` (the default branch).

- [ ] **Step 3: Write minimal implementation**

Edit `api/handlers_config_schema.go`. Extend `shapeString`:

```go
func shapeString(s descriptor.SectionShape) string {
	switch s {
	case descriptor.ShapeScalar:
		return "scalar"
	case descriptor.ShapeKeyedMap:
		return "keyed_map"
	case descriptor.ShapeList:
		return "list"
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run the full api test suite**

```
cd /path/to/hermind && go test ./api/...
```

Expected: PASS including the new test, plus every Stage 4a/4b tripwire (`TestConfigSchema_EmitsKeyedMapShapeString`, `TestConfigSchema_IncludesStage4bSections`, `TestConfigSchema_OmitsShapeForMapSections`).

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config_schema.go api/handlers_config_schema_test.go
git commit -m "feat(api): shapeString emits \"list\" for ShapeList sections"
```

---

## Task 3: redact/preserve branches for `ShapeList`

**Files:**
- Modify: `api/handlers_config.go`
- Modify: `api/handlers_config_test.go`

Depends on Task 1. Also exercises `RedactSectionSecretsForTest` / `PreserveSectionSecretsForTest` helpers Stage 4b added.

- [ ] **Step 1: Write the failing tests**

Append to `api/handlers_config_test.go`:

```go
func TestConfigGet_RedactsListSecrets(t *testing.T) {
	// A ShapeList section redacts every element's secret fields. Each
	// element is a map[string]any; redact walks the [] and blanks the
	// FieldSecret entries.
	defer descriptor.Unregister("__test_list_redact")
	descriptor.Register(descriptor.Section{
		Key:     "__test_list_redact",
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeList,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Kind: descriptor.FieldEnum, Required: true, Enum: []string{"anthropic"}},
			{Name: "api_key", Kind: descriptor.FieldSecret, Required: true},
		},
	})

	in := map[string]any{
		"__test_list_redact": []any{
			map[string]any{"provider": "anthropic", "api_key": "sk-one"},
			map[string]any{"provider": "anthropic", "api_key": "sk-two"},
		},
	}
	RedactSectionSecretsForTest(in)

	got := in["__test_list_redact"].([]any)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for i, raw := range got {
		inner := raw.(map[string]any)
		if inner["api_key"] != "" {
			t.Errorf("element %d api_key = %q, want blank", i, inner["api_key"])
		}
		if inner["provider"] != "anthropic" {
			t.Errorf("element %d provider mutated: %q", i, inner["provider"])
		}
	}
}

func TestConfigPut_PreservesListSecretsByIndex(t *testing.T) {
	// Preserve is strictly by index: updated[i].api_key == "" AND
	// current[i] has a non-empty api_key → restore current[i].api_key.
	// If lengths differ and updated has no current at index i, leave blank.
	defer descriptor.Unregister("__test_list_preserve")
	descriptor.Register(descriptor.Section{
		Key:     "__test_list_preserve",
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeList,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Kind: descriptor.FieldEnum, Required: true, Enum: []string{"anthropic", "openai"}},
			{Name: "api_key", Kind: descriptor.FieldSecret, Required: true},
		},
	})

	updated := map[string]any{
		"__test_list_preserve": []any{
			map[string]any{"provider": "anthropic", "api_key": ""},      // present in current → restore
			map[string]any{"provider": "openai", "api_key": "sk-new"},   // user retyped → keep
			map[string]any{"provider": "anthropic", "api_key": ""},      // appended; current[2] absent → stay blank
		},
	}
	current := map[string]any{
		"__test_list_preserve": []any{
			map[string]any{"provider": "anthropic", "api_key": "sk-zero"},
			map[string]any{"provider": "openai", "api_key": "sk-one"},
		},
	}
	PreserveSectionSecretsForTest(updated, current)

	got := updated["__test_list_preserve"].([]any)
	if got[0].(map[string]any)["api_key"] != "sk-zero" {
		t.Errorf("[0] api_key = %q, want %q", got[0].(map[string]any)["api_key"], "sk-zero")
	}
	if got[1].(map[string]any)["api_key"] != "sk-new" {
		t.Errorf("[1] api_key = %q, want preserved", got[1].(map[string]any)["api_key"])
	}
	if got[2].(map[string]any)["api_key"] != "" {
		t.Errorf("[2] api_key = %q, want blank (no current counterpart)", got[2].(map[string]any)["api_key"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd /path/to/hermind && go test ./api/... -run "TestConfigGet_RedactsListSecrets|TestConfigPut_PreservesListSecretsByIndex"
```

Expected: FAIL. The redact walk treats unknown shapes as maps (returns early); preserve does nothing for `ShapeList`.

- [ ] **Step 3: Write minimal implementation**

Edit `api/handlers_config.go`.

Inside `redactSectionSecrets`, add a new branch for `ShapeList` before the default (map) branch:

```go
if sec.Shape == descriptor.ShapeList {
	// Walk []any of elements, each itself map[string]any.
	outer, ok := m[sec.Key].([]any)
	if !ok {
		continue
	}
	for _, raw := range outer {
		inner, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, f := range sec.Fields {
			if f.Kind != descriptor.FieldSecret {
				continue
			}
			if _, present := inner[f.Name]; present {
				inner[f.Name] = ""
			}
		}
	}
	continue
}
```

Inside `preserveSectionSecrets`, add a new branch for `ShapeList` mirroring the `ShapeKeyedMap` branch but indexing with `int` instead of a string key:

```go
if sec.Shape == descriptor.ShapeList {
	outer, ok := updM[sec.Key].([]any)
	if !ok {
		continue
	}
	curOuter, _ := curM[sec.Key].([]any)
	for i, raw := range outer {
		inner, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if i >= len(curOuter) {
			// Appended element — no prior state to preserve.
			continue
		}
		curInst, _ := curOuter[i].(map[string]any)
		if curInst == nil {
			continue
		}
		for _, f := range sec.Fields {
			if f.Kind != descriptor.FieldSecret {
				continue
			}
			newVal, _ := inner[f.Name].(string)
			if newVal != "" {
				continue
			}
			prevVal, _ := curInst[f.Name].(string)
			if prevVal == "" {
				continue
			}
			inner[f.Name] = prevVal
			changed = true
		}
	}
	continue
}
```

Also extend `PreserveSectionSecretsForTest` at the bottom of the file to handle `ShapeList` with the same by-index walk, so the test harness can exercise the list path without a full YAML round-trip.

- [ ] **Step 4: Run tests to verify they pass**

```
cd /path/to/hermind && go test ./api/...
```

Expected: PASS (all including the two new tests + the Stage 4a/4b tripwires).

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go
git commit -m "feat(api): redact/preserve ShapeList elements by index"
```

---

## Task 4: `fallback_providers` descriptor

**Files:**
- Create: `config/descriptor/fallback_providers.go`
- Create: `config/descriptor/fallback_providers_test.go`

Depends on Task 1. Mirrors `providers.go` from Stage 4b.

- [ ] **Step 1: Write the failing test**

Create `config/descriptor/fallback_providers_test.go`:

```go
package descriptor

import (
	"strings"
	"testing"
)

func TestFallbackProvidersSection_ShapeAndFields(t *testing.T) {
	sec, ok := Get("fallback_providers")
	if !ok {
		t.Fatal("fallback_providers not registered")
	}
	if sec.GroupID != "models" {
		t.Errorf("GroupID = %q, want %q", sec.GroupID, "models")
	}
	if sec.Shape != ShapeList {
		t.Errorf("Shape = %d, want ShapeList (%d)", sec.Shape, ShapeList)
	}
	if sec.Label == "" {
		t.Error("Label is empty")
	}
	if sec.Summary == "" {
		t.Error("Summary is empty")
	}

	want := []struct {
		name string
		kind FieldKind
	}{
		{"provider", FieldEnum},
		{"base_url", FieldString},
		{"api_key", FieldSecret},
		{"model", FieldString},
	}
	if len(sec.Fields) != len(want) {
		t.Fatalf("field count = %d, want %d", len(sec.Fields), len(want))
	}
	for i, w := range want {
		if sec.Fields[i].Name != w.name {
			t.Errorf("[%d].Name = %q, want %q", i, sec.Fields[i].Name, w.name)
		}
		if sec.Fields[i].Kind != w.kind {
			t.Errorf("[%d].Kind = %v, want %v", i, sec.Fields[i].Kind, w.kind)
		}
	}
}

func TestFallbackProvidersSection_MirrorProviders(t *testing.T) {
	// Sanity: same field schema as the primary providers section. If 4b's
	// providers descriptor adds a field (e.g. "organization"), 4c's
	// fallback descriptor should follow — they describe the same struct.
	prim, ok := Get("providers")
	if !ok {
		t.Skip("providers not registered — 4b regression")
	}
	fb, ok := Get("fallback_providers")
	if !ok {
		t.Fatal("fallback_providers not registered")
	}
	if len(prim.Fields) != len(fb.Fields) {
		t.Fatalf("fallback field count diverged from primary: primary=%d fallback=%d",
			len(prim.Fields), len(fb.Fields))
	}
	for i := range prim.Fields {
		if prim.Fields[i].Name != fb.Fields[i].Name {
			t.Errorf("[%d] name divergence: primary=%q fallback=%q",
				i, prim.Fields[i].Name, fb.Fields[i].Name)
		}
		if prim.Fields[i].Kind != fb.Fields[i].Kind {
			t.Errorf("[%d] kind divergence: primary=%v fallback=%v",
				i, prim.Fields[i].Kind, fb.Fields[i].Kind)
		}
	}
}

func TestFallbackProvidersSection_ProviderEnumNonEmpty(t *testing.T) {
	sec, _ := Get("fallback_providers")
	provider := sec.Fields[0]
	if provider.Name != "provider" || provider.Kind != FieldEnum {
		t.Fatal("field 0 is not the provider enum")
	}
	if len(provider.Enum) == 0 {
		t.Error("provider Enum empty — factory.Types() returned nothing")
	}
	for _, got := range provider.Enum {
		if strings.TrimSpace(got) != got || got == "" {
			t.Errorf("provider enum entry %q has whitespace or is blank", got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /path/to/hermind && go test ./config/descriptor/... -run TestFallbackProviders
```

Expected: FAIL (section not registered).

- [ ] **Step 3: Write minimal implementation**

Create `config/descriptor/fallback_providers.go`:

```go
package descriptor

import "github.com/odysseythink/hermind/provider/factory"

// FallbackProviders mirrors config.Config.FallbackProviders ([]config.ProviderConfig).
// Every element conforms to the same 4-field schema as the primary Providers
// section (see providers.go) — the only structural difference is ordering:
// the runtime tries each fallback in list order.
func init() {
	Register(Section{
		Key:     "fallback_providers",
		Label:   "Fallback Providers",
		Summary: "Ordered list of providers tried in turn when the primary fails.",
		GroupID: "models",
		Shape:   ShapeList,
		Fields: []FieldSpec{
			{
				Name:     "provider",
				Label:    "Provider type",
				Kind:     FieldEnum,
				Required: true,
				Enum:     factory.Types(),
			},
			{
				Name:  "base_url",
				Label: "Base URL",
				Kind:  FieldString,
			},
			{
				Name:     "api_key",
				Label:    "API key",
				Kind:     FieldSecret,
				Required: true,
			},
			{
				Name:  "model",
				Label: "Model",
				Help:  "Optional — provider-qualified id used when this fallback is active.",
				Kind:  FieldString,
			},
		},
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
cd /path/to/hermind && go test ./config/descriptor/... ./api/...
```

Expected: PASS. Also pin the section in the api schema test — append to `api/handlers_config_schema_test.go`:

```go
func TestConfigSchema_IncludesStage4cSections(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.handleConfigSchema(rr, httptest.NewRequest(http.MethodGet, "/api/config/schema", nil))

	var resp ConfigSchemaResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var fb *ConfigSectionDTO
	for i := range resp.Sections {
		if resp.Sections[i].Key == "fallback_providers" {
			fb = &resp.Sections[i]
			break
		}
	}
	if fb == nil {
		t.Fatal("fallback_providers section missing from /api/config/schema")
	}
	if fb.Shape != "list" {
		t.Errorf("shape = %q, want %q", fb.Shape, "list")
	}
	if fb.GroupID != "models" {
		t.Errorf("GroupID = %q, want %q", fb.GroupID, "models")
	}
}
```

Re-run the api suite to confirm this pins.

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/fallback_providers.go config/descriptor/fallback_providers_test.go api/handlers_config_schema_test.go
git commit -m "feat(config/descriptor): FallbackProviders section (ShapeList, 4-field schema)"
```

---

## Task 5: Frontend Zod schema extension

**Files:**
- Modify: `web/src/api/schemas.ts`
- Modify: `web/src/api/schemas.test.ts`

- [ ] **Step 1: Write the failing tests**

Append to `web/src/api/schemas.test.ts`:

```ts
describe('ConfigSectionSchema — list shape', () => {
  it('accepts shape: "list"', () => {
    const parsed = ConfigSectionSchema.parse({
      key: 'fallback_providers',
      label: 'Fallback Providers',
      group_id: 'models',
      shape: 'list',
      fields: [
        { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
          enum: ['anthropic', 'openai'] },
      ],
    });
    expect(parsed.shape).toBe('list');
  });

  it('rejects unknown shape strings', () => {
    expect(() =>
      ConfigSectionSchema.parse({
        key: 'fallback_providers',
        label: 'X',
        group_id: 'models',
        shape: 'bogus_shape',
        fields: [],
      }),
    ).toThrow();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd web && pnpm vitest run src/api/schemas.test.ts -t "list shape"
```

Expected: FAIL — current `shape` enum is `z.enum(['map', 'scalar', 'keyed_map'])`, so `'list'` is rejected.

- [ ] **Step 3: Write minimal implementation**

Edit `web/src/api/schemas.ts`. Extend the enum:

```ts
shape: z.enum(['map', 'scalar', 'keyed_map', 'list']).optional(), // default (absent) = map
```

Leave every other field alone. No other schema change is required for 4c — the existing `ConfigFieldSchema` already covers the 4 ProviderConfig fields.

- [ ] **Step 4: Run tests to verify they pass**

```
cd web && pnpm vitest run src/api/schemas.test.ts
```

Expected: PASS including all Stage 4a/4b schema tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/api/schemas.ts web/src/api/schemas.test.ts
git commit -m "feat(web): Zod schema shape enum adds \"list\""
```

---

## Task 6: State reducer — `list-instance/*` actions

**Files:**
- Modify: `web/src/state.ts`
- Modify: `web/src/state.test.ts`

Five new actions:
1. `list-instance/create` — append a new element.
2. `list-instance/delete` — remove element at index; shift rest down.
3. `edit/list-instance-field` — set one field on element at index.
4. `list-instance/move-up` — swap with index-1 (no-op if index==0).
5. `list-instance/move-down` — swap with index+1 (no-op if index==len-1).

- [ ] **Step 1: Write the failing tests**

Append to `web/src/state.test.ts`:

```ts
describe('reducer: edit/list-instance-field', () => {
  it('updates one field on element at index', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [
          { provider: 'anthropic', api_key: '' },
          { provider: 'openai', api_key: '' },
        ],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [
          { provider: 'anthropic', api_key: '' },
          { provider: 'openai', api_key: '' },
        ],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'edit/list-instance-field',
      sectionKey: 'fallback_providers',
      index: 1,
      field: 'api_key',
      value: 'sk-new',
    });
    const list = (next.config as any).fallback_providers as Array<Record<string, unknown>>;
    expect(list[1].api_key).toBe('sk-new');
    expect(list[0].api_key).toBe(''); // untouched
  });

  it('is a no-op when index is out of bounds', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [{ provider: 'anthropic', api_key: '' }],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [{ provider: 'anthropic', api_key: '' }],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'edit/list-instance-field',
      sectionKey: 'fallback_providers',
      index: 5,
      field: 'api_key',
      value: 'sk-new',
    });
    expect(next).toBe(state);
  });
});

describe('reducer: list-instance/create', () => {
  it('appends the new element at the end', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [{ provider: 'anthropic', api_key: '' }],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [{ provider: 'anthropic', api_key: '' }],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'list-instance/create',
      sectionKey: 'fallback_providers',
      initial: { provider: 'openai', base_url: '', api_key: '', model: '' },
    });
    const list = (next.config as any).fallback_providers as Array<Record<string, unknown>>;
    expect(list).toHaveLength(2);
    expect(list[1].provider).toBe('openai');
  });

  it('initializes the list when the section is absent', () => {
    const state = { ...initialState, status: 'ready' as const };
    const next = reducer(state, {
      type: 'list-instance/create',
      sectionKey: 'fallback_providers',
      initial: { provider: 'anthropic', base_url: '', api_key: '', model: '' },
    });
    const list = (next.config as any).fallback_providers as unknown[];
    expect(list).toHaveLength(1);
  });
});

describe('reducer: list-instance/delete', () => {
  it('removes the element at index and shifts the rest', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
          { provider: 'anthropic', api_key: 'c' },
        ],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
          { provider: 'anthropic', api_key: 'c' },
        ],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'list-instance/delete',
      sectionKey: 'fallback_providers',
      index: 1,
    });
    const list = (next.config as any).fallback_providers as Array<Record<string, unknown>>;
    expect(list).toHaveLength(2);
    expect(list[0].provider).toBe('anthropic');
    expect(list[1].api_key).toBe('c');
  });

  it('is a no-op when index is out of bounds', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [{ provider: 'anthropic', api_key: '' }],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [{ provider: 'anthropic', api_key: '' }],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'list-instance/delete',
      sectionKey: 'fallback_providers',
      index: 9,
    });
    expect(next).toBe(state);
  });
});

describe('reducer: list-instance/move-up', () => {
  it('swaps element with its predecessor', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'list-instance/move-up',
      sectionKey: 'fallback_providers',
      index: 1,
    });
    const list = (next.config as any).fallback_providers as Array<Record<string, unknown>>;
    expect(list[0].provider).toBe('openai');
    expect(list[1].provider).toBe('anthropic');
  });

  it('is a no-op at index 0', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [{ provider: 'anthropic', api_key: 'a' }],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [{ provider: 'anthropic', api_key: 'a' }],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'list-instance/move-up',
      sectionKey: 'fallback_providers',
      index: 0,
    });
    expect(next).toBe(state);
  });
});

describe('reducer: list-instance/move-down', () => {
  it('swaps element with its successor', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'list-instance/move-down',
      sectionKey: 'fallback_providers',
      index: 0,
    });
    const list = (next.config as any).fallback_providers as Array<Record<string, unknown>>;
    expect(list[0].provider).toBe('openai');
    expect(list[1].provider).toBe('anthropic');
  });

  it('is a no-op at the last index', () => {
    const state = {
      ...initialState,
      status: 'ready' as const,
      config: {
        fallback_providers: [{ provider: 'anthropic', api_key: 'a' }],
      } as unknown as Config,
      originalConfig: {
        fallback_providers: [{ provider: 'anthropic', api_key: 'a' }],
      } as unknown as Config,
    };
    const next = reducer(state, {
      type: 'list-instance/move-down',
      sectionKey: 'fallback_providers',
      index: 0,
    });
    expect(next).toBe(state);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd web && pnpm vitest run src/state.test.ts -t "list-instance"
```

Expected: FAIL (every describe — actions and reducer cases don't exist).

- [ ] **Step 3: Write minimal implementation**

Edit `web/src/state.ts`. Extend the `Action` union:

```ts
  | { type: 'edit/list-instance-field'; sectionKey: string; index: number; field: string; value: unknown }
  | { type: 'list-instance/create'; sectionKey: string; initial: Record<string, unknown> }
  | { type: 'list-instance/delete'; sectionKey: string; index: number }
  | { type: 'list-instance/move-up'; sectionKey: string; index: number }
  | { type: 'list-instance/move-down'; sectionKey: string; index: number };
```

Add five reducer cases in the same ready-state switch that hosts the `edit/keyed-instance-field` family. Pattern each as:

```ts
    case 'edit/list-instance-field': {
      const list = ((state.config as Record<string, unknown>)[action.sectionKey] as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      if (action.index < 0 || action.index >= list.length) {
        return state;
      }
      const nextList = list.slice();
      nextList[action.index] = { ...nextList[action.index], [action.field]: action.value };
      return {
        ...state,
        config: { ...state.config, [action.sectionKey]: nextList } as typeof state.config,
      };
    }

    case 'list-instance/create': {
      const list = ((state.config as Record<string, unknown>)[action.sectionKey] as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      const nextList = list.concat([{ ...action.initial }]);
      return {
        ...state,
        config: { ...state.config, [action.sectionKey]: nextList } as typeof state.config,
      };
    }

    case 'list-instance/delete': {
      const list = ((state.config as Record<string, unknown>)[action.sectionKey] as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      if (action.index < 0 || action.index >= list.length) {
        return state;
      }
      const nextList = list.slice();
      nextList.splice(action.index, 1);
      return {
        ...state,
        config: { ...state.config, [action.sectionKey]: nextList } as typeof state.config,
      };
    }

    case 'list-instance/move-up':
    case 'list-instance/move-down': {
      const list = ((state.config as Record<string, unknown>)[action.sectionKey] as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      const target = action.type === 'list-instance/move-up' ? action.index - 1 : action.index + 1;
      if (action.index < 0 || action.index >= list.length) return state;
      if (target < 0 || target >= list.length) return state;
      const nextList = list.slice();
      [nextList[action.index], nextList[target]] = [nextList[target], nextList[action.index]];
      return {
        ...state,
        config: { ...state.config, [action.sectionKey]: nextList } as typeof state.config,
      };
    }
```

- [ ] **Step 4: Run tests to verify they pass**

```
cd web && pnpm vitest run src/state.test.ts
```

Expected: PASS including every prior reducer test.

- [ ] **Step 5: Commit**

```bash
git add web/src/state.ts web/src/state.test.ts
git commit -m "feat(web/state): list-instance actions (create, delete, edit-field, move-up/down)"
```

---

## Task 7: `listInstanceDirty` selector

**Files:**
- Create: `web/src/shell/listInstances.ts`
- Create: `web/src/shell/listInstances.test.ts`

Mirror `keyedInstanceDirty` from Stage 4b but index-addressed.

- [ ] **Step 1: Write the failing test**

Create `web/src/shell/listInstances.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { listInstanceDirty } from './listInstances';
import { initialState } from '../state';
import type { Config } from '../api/schemas';

function makeState(cfg: Config, orig: Config) {
  return { ...initialState, status: 'ready' as const, config: cfg, originalConfig: orig };
}

describe('listInstanceDirty', () => {
  it('returns false when element is identical to original', () => {
    const s = makeState(
      { fallback_providers: [{ provider: 'anthropic', api_key: 'a' }] } as unknown as Config,
      { fallback_providers: [{ provider: 'anthropic', api_key: 'a' }] } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(false);
  });

  it('returns true when a field differs', () => {
    const s = makeState(
      { fallback_providers: [{ provider: 'anthropic', api_key: 'b' }] } as unknown as Config,
      { fallback_providers: [{ provider: 'anthropic', api_key: 'a' }] } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(true);
  });

  it('returns true for appended elements (no original counterpart)', () => {
    const s = makeState(
      {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
      {
        fallback_providers: [{ provider: 'anthropic', api_key: 'a' }],
      } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 1)).toBe(true);
  });

  it('returns true after a reorder (index-based comparison)', () => {
    const s = makeState(
      {
        fallback_providers: [
          { provider: 'openai', api_key: 'b' },
          { provider: 'anthropic', api_key: 'a' },
        ],
      } as unknown as Config,
      {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(true);
    expect(listInstanceDirty(s, 'fallback_providers', 1)).toBe(true);
  });

  it('returns false when section is absent in both', () => {
    const s = makeState({} as Config, {} as Config);
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```
cd web && pnpm vitest run src/shell/listInstances.test.ts
```

Expected: FAIL — module does not exist yet.

- [ ] **Step 3: Write minimal implementation**

Create `web/src/shell/listInstances.ts`:

```ts
import type { AppState } from '../state';

/**
 * listInstanceDirty compares one element of a ShapeList section between
 * state.config and state.originalConfig. Addressing is strictly by index —
 * reordering the list dirties every row that moved.
 */
export function listInstanceDirty(
  state: AppState,
  sectionKey: string,
  index: number,
): boolean {
  const cur = ((state.config as Record<string, unknown>)[sectionKey] as
    | Array<Record<string, unknown>>
    | undefined) ?? [];
  const orig = ((state.originalConfig as Record<string, unknown>)[sectionKey] as
    | Array<Record<string, unknown>>
    | undefined) ?? [];
  const a = cur[index];
  const b = orig[index];
  if (a === b) return false;
  if (!a && !b) return false;
  if (!a || !b) return true;
  const keys = new Set<string>([...Object.keys(a), ...Object.keys(b)]);
  for (const k of keys) {
    if (a[k] !== b[k]) return true;
  }
  return false;
}
```

- [ ] **Step 4: Run test to verify it passes**

```
cd web && pnpm vitest run src/shell/listInstances.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/listInstances.ts web/src/shell/listInstances.test.ts
git commit -m "feat(web/shell): listInstanceDirty selector for ShapeList sections"
```

---

## Task 8: `FallbackProviderEditor` component

**Files:**
- Create: `web/src/components/groups/models/FallbackProviderEditor.tsx`
- Create: `web/src/components/groups/models/FallbackProviderEditor.module.css`
- Create: `web/src/components/groups/models/FallbackProviderEditor.test.tsx`

Thin shell around `ConfigSection` that adds position header, Delete, Up, Down. No fetch-models button (out of scope). No datalist (scope-out).

- [ ] **Step 1: Write failing tests**

Create `web/src/components/groups/models/FallbackProviderEditor.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FallbackProviderEditor from './FallbackProviderEditor';
import type { ConfigSection } from '../../../api/schemas';

const section: ConfigSection = {
  key: 'fallback_providers',
  label: 'Fallback Providers',
  group_id: 'models',
  shape: 'list',
  fields: [
    { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
      enum: ['anthropic', 'openai'] },
    { name: 'base_url', label: 'Base URL', kind: 'string' },
    { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    { name: 'model', label: 'Model', kind: 'string' },
  ],
};

function baseProps(overrides: Partial<React.ComponentProps<typeof FallbackProviderEditor>> = {}) {
  return {
    sectionKey: 'fallback_providers',
    index: 0,
    length: 1,
    section,
    value: { provider: 'anthropic', base_url: '', api_key: '', model: '' },
    originalValue: { provider: 'anthropic', base_url: '', api_key: '', model: '' },
    dirty: false,
    onField: vi.fn(),
    onDelete: vi.fn(),
    onMoveUp: vi.fn(),
    onMoveDown: vi.fn(),
    ...overrides,
  };
}

describe('FallbackProviderEditor', () => {
  it('renders a position header "Fallback #1" for index 0', () => {
    render(<FallbackProviderEditor {...baseProps()} />);
    expect(screen.getByText(/fallback #1/i)).toBeInTheDocument();
  });

  it('renders all 4 fields', () => {
    render(<FallbackProviderEditor {...baseProps()} />);
    expect(screen.getByLabelText(/provider type/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/base url/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^model/i)).toBeInTheDocument();
  });

  it('disables Up at index 0', () => {
    render(<FallbackProviderEditor {...baseProps({ index: 0, length: 3 })} />);
    expect(screen.getByRole('button', { name: /move up/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /move down/i })).toBeEnabled();
  });

  it('disables Down at the last index', () => {
    render(<FallbackProviderEditor {...baseProps({ index: 2, length: 3 })} />);
    expect(screen.getByRole('button', { name: /move up/i })).toBeEnabled();
    expect(screen.getByRole('button', { name: /move down/i })).toBeDisabled();
  });

  it('calls onDelete after confirm', async () => {
    const onDelete = vi.fn();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<FallbackProviderEditor {...baseProps({ onDelete })} />);
    await userEvent.click(screen.getByRole('button', { name: /delete/i }));
    expect(onDelete).toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it('skips onDelete when confirm is denied', async () => {
    const onDelete = vi.fn();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(<FallbackProviderEditor {...baseProps({ onDelete })} />);
    await userEvent.click(screen.getByRole('button', { name: /delete/i }));
    expect(onDelete).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd web && pnpm vitest run src/components/groups/models/FallbackProviderEditor.test.tsx
```

Expected: FAIL — component file does not exist.

- [ ] **Step 3: Write the component**

Create `web/src/components/groups/models/FallbackProviderEditor.module.css` (styles matching ProviderEditor's visual weight — see existing `ProviderEditor.module.css` for the pattern). Keep it minimal: `.editor`, `.header`, `.breadcrumb`, `.body`, `.footer`, `.deleteBtn`, `.moveBtn`.

Create `web/src/components/groups/models/FallbackProviderEditor.tsx`:

```tsx
import styles from './FallbackProviderEditor.module.css';
import ConfigSection from '../../ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../../../api/schemas';

export interface FallbackProviderEditorProps {
  sectionKey: string;
  index: number;
  length: number;
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  dirty: boolean;
  onField: (index: number, field: string, value: unknown) => void;
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
}

export default function FallbackProviderEditor(props: FallbackProviderEditorProps) {
  function onDeleteClick() {
    if (window.confirm(`Delete fallback #${props.index + 1}? This cannot be undone.`)) {
      props.onDelete();
    }
  }

  return (
    <section className={styles.editor}>
      <header className={styles.header}>
        <div className={styles.breadcrumb}>
          Models / Fallback Providers / <strong>Fallback #{props.index + 1}</strong>
        </div>
        <div className={styles.headerBtns}>
          <button
            type="button"
            className={styles.moveBtn}
            aria-label="Move up"
            disabled={props.index === 0}
            onClick={props.onMoveUp}
          >
            ↑
          </button>
          <button
            type="button"
            className={styles.moveBtn}
            aria-label="Move down"
            disabled={props.index === props.length - 1}
            onClick={props.onMoveDown}
          >
            ↓
          </button>
          <button type="button" className={styles.deleteBtn} onClick={onDeleteClick}>
            Delete
          </button>
        </div>
      </header>
      <div className={styles.body}>
        <ConfigSection
          section={props.section}
          value={props.value}
          originalValue={props.originalValue}
          onFieldChange={(field, v) => props.onField(props.index, field, v)}
        />
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
cd web && pnpm vitest run src/components/groups/models/FallbackProviderEditor.test.tsx
```

Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/groups/models/FallbackProviderEditor.tsx web/src/components/groups/models/FallbackProviderEditor.module.css web/src/components/groups/models/FallbackProviderEditor.test.tsx
git commit -m "feat(web/models): FallbackProviderEditor component"
```

---

## Task 9: Extend ModelsSidebar with fallback rows

**Files:**
- Modify: `web/src/components/groups/models/ModelsSidebar.tsx`
- Modify: `web/src/components/groups/models/ModelsSidebar.module.css`
- Modify: `web/src/components/groups/models/ModelsSidebar.test.tsx`

ModelsSidebar gets a third section below Providers:

```
[Default model]
[Providers]
  anthropic_main
  + New provider
[Fallback Providers]
  #1 anthropic ↑↓
  #2 openai ↑↓
  + Add fallback
```

Each fallback row is a button opening the main-pane editor. Up/Down buttons inline on the row fire the reorder action WITHOUT changing selection. "+ Add fallback" dispatches `list-instance/create` with an empty initial (provider = first enum value) and selects the new row.

- [ ] **Step 1: Write failing tests**

Append a new describe to `web/src/components/groups/models/ModelsSidebar.test.tsx`:

```tsx
describe('ModelsSidebar — fallback providers section', () => {
  it('renders the "Fallback Providers" header', () => {
    render(<ModelsSidebar {...baseProps({ fallbackProviders: [] })} />);
    expect(screen.getByText(/fallback providers/i)).toBeInTheDocument();
  });

  it('renders one row per fallback with #N position badge', () => {
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [
            { provider: 'anthropic' },
            { provider: 'openai' },
          ],
        })}
      />,
    );
    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
    expect(screen.getByText('anthropic')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
  });

  it('disables up/down at boundaries', () => {
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
        })}
      />,
    );
    const upButtons = screen.getAllByRole('button', { name: /move up/i });
    const downButtons = screen.getAllByRole('button', { name: /move down/i });
    expect(upButtons[0]).toBeDisabled();
    expect(downButtons[0]).toBeEnabled();
    expect(upButtons[1]).toBeEnabled();
    expect(downButtons[1]).toBeDisabled();
  });

  it('renders "+ Add fallback" button', () => {
    render(<ModelsSidebar {...baseProps({ fallbackProviders: [] })} />);
    expect(screen.getByRole('button', { name: /add fallback/i })).toBeInTheDocument();
  });

  it('calls onAddFallback when clicked', async () => {
    const onAddFallback = vi.fn();
    render(<ModelsSidebar {...baseProps({ fallbackProviders: [], onAddFallback })} />);
    await userEvent.click(screen.getByRole('button', { name: /add fallback/i }));
    expect(onAddFallback).toHaveBeenCalled();
  });

  it('calls onSelectFallback with the index when a row is clicked', async () => {
    const onSelectFallback = vi.fn();
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
          onSelectFallback,
        })}
      />,
    );
    await userEvent.click(screen.getByText('openai'));
    expect(onSelectFallback).toHaveBeenCalledWith(1);
  });

  it('calls onMoveFallback(i, "up") / ("down") on the inline buttons', async () => {
    const onMoveFallback = vi.fn();
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
          onMoveFallback,
        })}
      />,
    );
    await userEvent.click(screen.getAllByRole('button', { name: /move down/i })[0]);
    expect(onMoveFallback).toHaveBeenCalledWith(0, 'down');
    await userEvent.click(screen.getAllByRole('button', { name: /move up/i })[1]);
    expect(onMoveFallback).toHaveBeenCalledWith(1, 'up');
  });
});
```

Update the existing `baseProps` helper at the top of the test file to default the new props:

```ts
fallbackProviders: [] as Array<{ provider: string }>,
dirtyFallbackIndices: new Set<number>(),
activeFallbackIndex: null as number | null,
onSelectFallback: vi.fn(),
onAddFallback: vi.fn(),
onMoveFallback: vi.fn(),
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd web && pnpm vitest run src/components/groups/models/ModelsSidebar.test.tsx
```

Expected: FAIL on every new case (props/UI not yet wired).

- [ ] **Step 3: Write the implementation**

Edit `web/src/components/groups/models/ModelsSidebar.tsx`. Extend `ModelsSidebarProps`:

```ts
export interface ModelsSidebarProps {
  instances: Array<{ key: string; type: string }>;
  activeSubKey: string | null;
  dirtyKeys: Set<string>;
  onSelectScalar: (key: string) => void;
  onSelectInstance: (key: string) => void;
  onNewProvider: () => void;
  // Stage 4c additions
  fallbackProviders: Array<{ provider: string }>;
  dirtyFallbackIndices: Set<number>;
  activeFallbackIndex: number | null;
  onSelectFallback: (index: number) => void;
  onAddFallback: () => void;
  onMoveFallback: (index: number, direction: 'up' | 'down') => void;
}
```

Append the fallback section after the existing Providers block:

```tsx
      <div className={styles.groupHeader}>Fallback Providers</div>
      {props.fallbackProviders.length === 0 && (
        <div className={styles.empty}>No fallback providers configured.</div>
      )}
      {props.fallbackProviders.map((fb, i) => {
        const active = i === props.activeFallbackIndex;
        const atTop = i === 0;
        const atBottom = i === props.fallbackProviders.length - 1;
        return (
          <div key={i} className={`${styles.fallbackRow} ${active ? styles.active : ''}`}>
            <button
              type="button"
              className={styles.fallbackBody}
              onClick={() => props.onSelectFallback(i)}
            >
              <span className={styles.posBadge}>#{i + 1}</span>
              <span className={styles.fallbackType}>{fb.provider}</span>
              {props.dirtyFallbackIndices.has(i) && (
                <span className={styles.dirtyDot} title="Unsaved changes" />
              )}
            </button>
            <div className={styles.fallbackMoveBtns}>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label="Move up"
                disabled={atTop}
                onClick={() => props.onMoveFallback(i, 'up')}
              >
                ↑
              </button>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label="Move down"
                disabled={atBottom}
                onClick={() => props.onMoveFallback(i, 'down')}
              >
                ↓
              </button>
            </div>
          </div>
        );
      })}
      <button type="button" className={styles.newBtn} onClick={props.onAddFallback}>
        + Add fallback
      </button>
```

Edit `ModelsSidebar.module.css` to add the new class names (`.fallbackRow`, `.fallbackBody`, `.posBadge`, `.fallbackType`, `.fallbackMoveBtns`, `.moveBtn`). Keep the visual language consistent with the existing Providers rows — position badge is a small muted pill, type name is the primary text, move buttons are a tight vertical stack on the right.

- [ ] **Step 4: Run tests to verify they pass**

```
cd web && pnpm vitest run src/components/groups/models/ModelsSidebar.test.tsx
```

Expected: PASS (5 existing + 7 new = 12).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/groups/models/ModelsSidebar.tsx web/src/components/groups/models/ModelsSidebar.module.css web/src/components/groups/models/ModelsSidebar.test.tsx
git commit -m "feat(web/models): ModelsSidebar renders Fallback Providers section"
```

---

## Task 10: Integration — sections.ts, ContentPanel, Sidebar, App.tsx

**Files:**
- Modify: `web/src/shell/sections.ts`
- Modify: `web/src/shell/sections.test.ts`
- Modify: `web/src/components/shell/ContentPanel.tsx`
- Modify: `web/src/components/shell/ContentPanel.test.tsx`
- Modify: `web/src/components/shell/Sidebar.tsx`
- Modify: `web/src/components/shell/Sidebar.test.tsx`
- Modify: `web/src/App.tsx`

Hash scheme: `#models/fallback:N` addresses the N-th fallback (zero-indexed). ContentPanel detects the `fallback:` prefix, parses the index, and routes to FallbackProviderEditor.

- [ ] **Step 1: Write failing tests**

Append to `web/src/shell/sections.test.ts`:

```ts
it('registers Stage 4c section: fallback_providers under models', () => {
  const fb = findSection('fallback_providers');
  expect(fb, 'missing fallback_providers').toBeDefined();
  expect(fb!.groupId).toBe('models');
  expect(fb!.plannedStage).toBe('done');
});

it('models group exposes model, providers, fallback_providers in order', () => {
  const models = sectionsInGroup('models');
  expect(models.map(s => s.key)).toEqual(['model', 'providers', 'fallback_providers']);
});
```

Replace the existing `'models group exposes model then providers in declaration order'` test to include the new entry — keep only one assertion on models-group order.

Append to `web/src/components/shell/ContentPanel.test.tsx`:

```tsx
describe('ContentPanel — list section routing', () => {
  const fbSection: ConfigSection = {
    key: 'fallback_providers',
    label: 'Fallback Providers',
    group_id: 'models',
    shape: 'list',
    fields: [
      { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
        enum: ['anthropic', 'openai'] },
      { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    ],
  };

  it('renders FallbackProviderEditor for a fallback:N subkey', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'fallback:1',
          config: {
            fallback_providers: [
              { provider: 'anthropic', api_key: '' },
              { provider: 'openai', api_key: '' },
            ],
          } as unknown as Config,
          originalConfig: {
            fallback_providers: [
              { provider: 'anthropic', api_key: '' },
              { provider: 'openai', api_key: '' },
            ],
          } as unknown as Config,
          configSections: [fbSection],
        })}
      />,
    );
    expect(screen.getByText(/fallback #2/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
  });

  it('falls back to EmptyState for a fallback:N with no matching element', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'fallback:5',
          config: { fallback_providers: [] } as unknown as Config,
          originalConfig: { fallback_providers: [] } as unknown as Config,
          configSections: [fbSection],
        })}
      />,
    );
    // ComingSoonPanel or EmptyState — either is acceptable as long as the
    // editor does NOT render (no "Fallback #" header).
    expect(screen.queryByText(/fallback #/i)).not.toBeInTheDocument();
  });
});
```

Extend `makeProps` in the same file to include:

```ts
onConfigListField: () => {},
onConfigListDelete: () => {},
onConfigListMove: () => {},
```

Also extend the explicit `baseProps` block in the "non-gateway section routing" describe with the same three defaults.

Append to `web/src/components/shell/Sidebar.test.tsx` — inside the `Sidebar — models group` describe, extend the existing `baseProps` and add an assertion:

```tsx
it('renders Fallback Providers rows for the models group', () => {
  render(
    <Sidebar
      {...baseProps({
        expandedGroups: new Set(['models']),
        fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
      })}
    />,
  );
  expect(screen.getByText(/fallback providers/i)).toBeInTheDocument();
  expect(screen.getByText('#1')).toBeInTheDocument();
  expect(screen.getByText('#2')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /add fallback/i })).toBeInTheDocument();
});
```

Update the `baseProps` helper at the top of Sidebar.test.tsx to default the new props:

```ts
fallbackProviders: [],
dirtyFallbackIndices: new Set<number>(),
onAddFallback: vi.fn(),
onMoveFallback: vi.fn(),
```

Every Sidebar.test.tsx test that passes props individually (not via `baseProps`) needs the same four defaults appended — match the pattern Stage 4b used when adding `providerInstances`, `dirtyProviderKeys`, `onNewProvider`.

- [ ] **Step 2: Run tests to verify they fail**

```
cd web && pnpm vitest run src/shell/sections.test.ts src/components/shell/ContentPanel.test.tsx src/components/shell/Sidebar.test.tsx
```

Expected: FAIL on the new cases. Sidebar compilation errors are acceptable until Step 5.

- [ ] **Step 3: Register fallback_providers in sections.ts**

Edit `web/src/shell/sections.ts`. Append after the `providers` entry:

```ts
  { key: 'fallback_providers', groupId: 'models', plannedStage: 'done' },
```

Final models section of `SECTIONS`:

```ts
  // models
  { key: 'model', groupId: 'models', plannedStage: 'done' },
  { key: 'providers', groupId: 'models', plannedStage: 'done' },
  { key: 'fallback_providers', groupId: 'models', plannedStage: 'done' },
```

- [ ] **Step 4: Route the `list` branch in ContentPanel**

Edit `web/src/components/shell/ContentPanel.tsx`. Import:

```tsx
import FallbackProviderEditor from '../groups/models/FallbackProviderEditor';
```

Extend `ContentPanelProps`:

```ts
  onConfigListField: (sectionKey: string, index: number, field: string, value: unknown) => void;
  onConfigListDelete: (sectionKey: string, index: number) => void;
  onConfigListMove: (sectionKey: string, index: number, direction: 'up' | 'down') => void;
```

Inside the `if (props.activeSubKey)` block, after the keyed-map fallthrough that tries the providers-instance path, add a fallback-index path:

```tsx
    // fallback:N addresses the N-th element of fallback_providers.
    if (props.activeGroup === 'models' && props.activeSubKey.startsWith('fallback:')) {
      const index = Number(props.activeSubKey.slice('fallback:'.length));
      const fbSection = props.configSections.find(s => s.key === 'fallback_providers');
      const list = ((props.config as Record<string, unknown>).fallback_providers as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      const origList = ((props.originalConfig as Record<string, unknown>).fallback_providers as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      if (
        fbSection &&
        fbSection.shape === 'list' &&
        Number.isInteger(index) &&
        index >= 0 &&
        index < list.length
      ) {
        const element = list[index];
        const originalElement = origList[index];
        const dirty = !shallowEqualInstance(element, originalElement);
        return (
          <FallbackProviderEditor
            sectionKey="fallback_providers"
            index={index}
            length={list.length}
            section={fbSection}
            value={element}
            originalValue={originalElement ?? {}}
            dirty={dirty}
            onField={(i, field, v) =>
              props.onConfigListField('fallback_providers', i, field, v)
            }
            onDelete={() => props.onConfigListDelete('fallback_providers', index)}
            onMoveUp={() => props.onConfigListMove('fallback_providers', index, 'up')}
            onMoveDown={() => props.onConfigListMove('fallback_providers', index, 'down')}
          />
        );
      }
    }
```

Also extend the section-matching branch so that a `fallback_providers` section key (without `:N`) triggers the EmptyState:

```tsx
      if (section.shape === 'keyed_map' || section.shape === 'list') {
        return <EmptyState onSelectGroup={props.onSelectGroup} />;
      }
```

Rename or reshape the existing `keyed_map` guard to include `list` — or add the line right after the existing keyed_map branch. Preserve the 4b behavior.

- [ ] **Step 5: Route models group in Sidebar**

Edit `web/src/components/shell/Sidebar.tsx`. Extend `SidebarProps`:

```ts
  fallbackProviders: Array<{ provider: string }>;
  dirtyFallbackIndices: Set<number>;
  onAddFallback: () => void;
  onMoveFallback: (index: number, direction: 'up' | 'down') => void;
```

Extend the `ModelsSidebar` invocation inside the `g.id === 'models'` branch to pass the new props:

```tsx
<ModelsSidebar
  instances={props.providerInstances}
  activeSubKey={props.activeGroup === 'models' ? props.activeSubKey : null}
  dirtyKeys={props.dirtyProviderKeys}
  onSelectScalar={key => {
    props.onSelectGroup('models');
    props.onSelectSub(key);
  }}
  onSelectInstance={key => {
    props.onSelectGroup('models');
    props.onSelectSub(key);
  }}
  onNewProvider={props.onNewProvider}
  fallbackProviders={props.fallbackProviders}
  dirtyFallbackIndices={props.dirtyFallbackIndices}
  activeFallbackIndex={(() => {
    if (props.activeGroup !== 'models') return null;
    if (!props.activeSubKey || !props.activeSubKey.startsWith('fallback:')) return null;
    const n = Number(props.activeSubKey.slice('fallback:'.length));
    return Number.isInteger(n) ? n : null;
  })()}
  onSelectFallback={i => {
    props.onSelectGroup('models');
    props.onSelectSub(`fallback:${i}`);
  }}
  onAddFallback={props.onAddFallback}
  onMoveFallback={props.onMoveFallback}
/>
```

- [ ] **Step 6: Wire App.tsx**

Edit `web/src/App.tsx`. After the `dirtyProviderKeys` memo, add:

```tsx
  const fallbackProviders = useMemo(() => {
    const list = ((state.config as Record<string, unknown>).fallback_providers as
      | Array<Record<string, unknown>>
      | undefined) ?? [];
    return list.map(item => ({ provider: (item.provider as string) ?? '' }));
  }, [state.config]);

  const dirtyFallbackIndices = useMemo(() => {
    const cur = ((state.config as Record<string, unknown>).fallback_providers as
      | Array<unknown>
      | undefined) ?? [];
    const orig = ((state.originalConfig as Record<string, unknown>).fallback_providers as
      | Array<unknown>
      | undefined) ?? [];
    const len = Math.max(cur.length, orig.length);
    const out = new Set<number>();
    for (let i = 0; i < len; i++) {
      if (listInstanceDirty(state, 'fallback_providers', i)) out.add(i);
    }
    return out;
  }, [state]);
```

Import:

```tsx
import { listInstanceDirty } from './shell/listInstances';
```

Pass the new props to `<Sidebar>`:

```tsx
fallbackProviders={fallbackProviders}
dirtyFallbackIndices={dirtyFallbackIndices}
onAddFallback={() => {
  const list = ((state.config as Record<string, unknown>).fallback_providers as
    | Array<unknown>
    | undefined) ?? [];
  const section = state.configSections.find(s => s.key === 'fallback_providers');
  const providerField = section?.fields.find(f => f.name === 'provider');
  const firstType = providerField?.enum?.[0] ?? '';
  dispatch({
    type: 'list-instance/create',
    sectionKey: 'fallback_providers',
    initial: { provider: firstType, base_url: '', api_key: '', model: '' },
  });
  dispatch({ type: 'shell/selectGroup', group: 'models' });
  dispatch({ type: 'shell/selectSub', key: `fallback:${list.length}` });
}}
onMoveFallback={(index, direction) =>
  dispatch({
    type: direction === 'up' ? 'list-instance/move-up' : 'list-instance/move-down',
    sectionKey: 'fallback_providers',
    index,
  })
}
```

Pass the new props to `<ContentPanel>`:

```tsx
onConfigListField={(sectionKey, index, field, value) =>
  dispatch({ type: 'edit/list-instance-field', sectionKey, index, field, value })
}
onConfigListDelete={(sectionKey, index) => {
  dispatch({ type: 'list-instance/delete', sectionKey, index });
  dispatch({ type: 'shell/selectSub', key: null });
}}
onConfigListMove={(sectionKey, index, direction) => {
  dispatch({
    type: direction === 'up' ? 'list-instance/move-up' : 'list-instance/move-down',
    sectionKey,
    index,
  });
  // Track the moved element's new index in the hash.
  const newIndex = direction === 'up' ? index - 1 : index + 1;
  dispatch({ type: 'shell/selectSub', key: `fallback:${newIndex}` });
}}
```

- [ ] **Step 7: Run the full web suite**

```
cd web && pnpm test && pnpm type-check && pnpm lint
```

Expected: all PASS. Test count jumps from 204 (post-Stage-4b) to ~240 (Stage 4c adds: 2 schema + 10 state + 5 listInstances + 6 FallbackProviderEditor + 7 ModelsSidebar + 2 ContentPanel + 1 Sidebar + 2 sections = 35 new, minus 1 removed sections-order test = 34 net). Any number outside 236-242 — stop and investigate.

- [ ] **Step 8: Commit**

```bash
git add web/src/shell/sections.ts web/src/shell/sections.test.ts web/src/components/shell/ContentPanel.tsx web/src/components/shell/ContentPanel.test.tsx web/src/components/shell/Sidebar.tsx web/src/components/shell/Sidebar.test.tsx web/src/App.tsx
git commit -m "feat(web): integrate Fallback Providers — Sidebar rows, ContentPanel routing, App dispatchers"
```

---

## Task 11: Gauntlet + webroot + smoke doc

**Files:**
- Rebuilt: `api/webroot/` (generated by `make web-check`)
- Modify: `docs/smoke/web-config.md`

- [ ] **Step 1: Run the Go test suite**

```
go test ./config/descriptor/... ./api/...
```

Expected: PASS on every package.

- [ ] **Step 2: Run the web test suite**

```
cd web && pnpm test
```

Expected: ~240 tests pass.

- [ ] **Step 3: Type-check + lint**

```
cd web && pnpm type-check && pnpm lint
```

Expected: both PASS with zero warnings.

- [ ] **Step 4: Rebuild the web bundle**

```
make web
```

Expected: Vite build succeeds; `api/webroot/` re-synced from `web/dist/`. (The full `make web-check` will complain about uncommitted webroot diffs — that's expected; we commit it in Step 6.)

- [ ] **Step 5: Append the Stage 4c smoke-doc section**

Append to `docs/smoke/web-config.md`:

```markdown
## Stage 4c · Fallback Providers editor

- Sidebar Models group now has a "Fallback Providers" section below Providers, with a `+ Add fallback` button.
- Click `+ Add fallback` — a new row `#1 <first-provider-type>` appears, URL becomes `#models/fallback:0`, and the main pane shows "Fallback #1" with the 4 fields (provider pre-selected as the first enum value).
- Edit provider type, base URL, API key, model. Save — toast "Saved". YAML `fallback_providers:` block now has one entry.
- Click `+ Add fallback` again. Second row `#2 <type>` appears. Click the `↑` on the `#2` row in the sidebar — rows swap in place; URL updates to `#models/fallback:0` tracking the moved element. Main-pane header now shows "Fallback #1" for what was previously #2.
- Click Delete on the main-pane editor, confirm → sidebar row disappears, pane returns to EmptyState. Save — YAML `fallback_providers` shrinks by one.
- Re-visit an existing fallback. API key field is blanked (GET redacts). Click Save without typing a new key — stored key at the same index is preserved (same contract as `providers.*.api_key`). **Gotcha:** after deleting or reordering fallbacks, the index-based preserve can misalign secrets. Always re-type the API key for any moved or deleted neighbor before saving.
- With zero fallback entries, the sidebar shows "No fallback providers configured." and the YAML omits the `fallback_providers` key (thanks to `omitempty`).
```

- [ ] **Step 6: Commit**

Stage ONLY `api/webroot/` and `docs/smoke/web-config.md`:

```bash
git add api/webroot/ docs/smoke/web-config.md
git commit -m "chore(web): rebuild webroot + Stage 4c smoke flow"
```

- [ ] **Step 7: Manual smoke (required before sign-off)**

Rebuild the binary so the new webroot is embedded:

```bash
go build -o bin/hermind ./cmd/hermind
./bin/hermind web --addr=127.0.0.1:9119 --no-browser
```

Hard-refresh the browser and work through every bullet from Step 5. Every bullet must pass before calling Stage 4c done.

---

## Completion checklist

Before calling Stage 4c complete, verify:

- [ ] `go test ./config/descriptor/... ./api/...` — PASS
- [ ] `cd web && pnpm test` — PASS (~240 tests)
- [ ] `cd web && pnpm type-check` — PASS
- [ ] `cd web && pnpm lint` — zero warnings
- [ ] `make web-check` — PASS
- [ ] Manual smoke from Task 11 Step 7 — all bullets pass
- [ ] `docs/smoke/web-config.md` has the new Stage 4c section
- [ ] `git status --short` — no dangling Stage-4c files (unrelated in-progress stages may remain)

Once all boxes are checked, Stage 4c is complete. Stage 4d candidates: fallback-aware fetch-models (per-index connection test), drag-and-drop reorder, cross-section datalist for Default Model autocomplete pulling from `providers.*.model`.
