# Web Config Schema Infrastructure (Stage 4b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Providers editor — a `map[string]ProviderConfig` section under the Models group — plus `POST /api/providers/{name}/models` that doubles as a fetch-models action and connection test.

**Architecture:** Stage 4a introduced `Section.Shape` with `ShapeScalar`. Stage 4b adds `ShapeKeyedMap` for map-of-uniform-struct sections. Each `ShapeKeyedMap` instance conforms to the same `Fields` schema; every instance's secret fields round-trip through the existing redact/preserve plumbing, just iterated per-instance. One new API handler ports the legacy fetch-models behavior into the Stage-1+ `api/` package. Three new React components under `web/src/components/groups/models/` render the sidebar list, single-instance editor, and new-instance modal.

**Tech Stack:** Go 1.21+ (config/descriptor, api, provider/factory), TypeScript + React + Zod (web/src), Vitest, Go `testing`, chi router, react-testing-library.

**Spec:** `docs/superpowers/specs/2026-04-20-web-schema-infra-stage4b-design.md`.

**Scope note — what this does NOT ship:**
- Fallback providers list (deferred to 4c).
- Cross-section datalist: Default Model and Auxiliary Model continue to render as plain text inputs. Autocomplete is scoped to the per-provider-instance Model field inside the new ProviderEditor.
- Separate test/fetch buttons. Fetch-models IS the connection test — one button, one endpoint, one network call.
- Preview-before-save (endpoint uses stored config; user must Save before Fetch).
- Connection-test-as-capability abstraction. The fetch-models button is hand-wired into ProviderEditor.

---

## File Structure

**Created (Go):**
- `config/descriptor/providers.go` — `ShapeKeyedMap` section registering the 4 ProviderConfig fields.
- `config/descriptor/providers_test.go` — field-kind assertions and provider-enum sanity floor.
- `api/handlers_providers_models.go` — `handleProvidersModels` serving `POST /api/providers/{name}/models`.
- `api/handlers_providers_models_test.go` — 5 status-code cases using a `fakeProvider`/`fakeLister` harness.

**Modified (Go):**
- `config/descriptor/descriptor.go` — add `ShapeKeyedMap` const.
- `config/descriptor/descriptor_test.go` — invariant extension + inline-seeded failure test.
- `api/handlers_config_schema.go` — `shapeString` returns `"keyed_map"` for the new shape.
- `api/handlers_config.go` — add ShapeKeyedMap branch in `redactSectionSecrets` and `preserveSectionSecrets`.
- `api/handlers_config_schema_test.go` — new `TestConfigSchema_IncludesStage4bSections`.
- `api/handlers_config_test.go` — new `TestConfigGet_RedactsKeyedMapSecrets` + `TestConfigPut_PreservesKeyedMapSecrets` (round-trip checks for a keyed-map instance secret).
- `api/server.go` — mount the new route under the existing auth middleware.

**Created (TypeScript):**
- `web/src/components/groups/models/ModelsSidebar.tsx` — renders the Models group body: scalar `model` entry + provider instance list + "+ New provider" button.
- `web/src/components/groups/models/ModelsSidebar.module.css`.
- `web/src/components/groups/models/ModelsSidebar.test.tsx`.
- `web/src/components/groups/models/ProviderEditor.tsx` — main-pane editor for one provider instance (4 fields + Delete + Fetch models).
- `web/src/components/groups/models/ProviderEditor.module.css`.
- `web/src/components/groups/models/ProviderEditor.test.tsx`.
- `web/src/components/groups/models/NewProviderDialog.tsx` — modal for creating a provider instance with key + type.
- `web/src/components/groups/models/NewProviderDialog.module.css`.
- `web/src/components/groups/models/NewProviderDialog.test.tsx`.
- `web/src/shell/keyedInstances.ts` — `keyedInstanceDirty(sectionKey, instanceKey, state)` selector used by ModelsSidebar for the per-instance dirty dot.
- `web/src/shell/keyedInstances.test.ts`.

**Modified (TypeScript):**
- `web/src/api/schemas.ts` — `shape` enum adds `'keyed_map'`; new `ProviderModelsResponseSchema`.
- `web/src/api/schemas.test.ts` — parse/reject cases for `'keyed_map'` and the models response.
- `web/src/state.ts` — three new `keyed-instance/*` actions + reducer cases.
- `web/src/state.test.ts` — three new describe blocks for the reducer cases.
- `web/src/shell/sections.ts` — append `{ key: 'providers', groupId: 'models', plannedStage: 'done' }`.
- `web/src/shell/sections.test.ts` — models group order is `['model', 'providers']`.
- `web/src/components/shell/ContentPanel.tsx` — `keyed_map` branch routes to `ProviderEditor`.
- `web/src/components/shell/ContentPanel.test.tsx` — new describe block for the `keyed_map` routing.
- `web/src/components/shell/Sidebar.tsx` — route `g.id === 'models'` to `ModelsSidebar`.
- `web/src/components/shell/Sidebar.test.tsx` — new describe block for the Models group wiring.
- `web/src/App.tsx` — compute `providerInstances`, thread `onConfigProviderDialog` + `onConfigKeyed*` dispatchers, host a `newProviderDialogOpen` state hook.
- `docs/smoke/web-config.md` — append `## Stage 4b · Providers editor + fetch-models`.

**Rebuilt (generated):**
- `api/webroot/` — regenerated by `make web-check`.

---

## Task 1: `ShapeKeyedMap` constant + invariant

**Files:**
- Modify: `config/descriptor/descriptor.go`
- Modify: `config/descriptor/descriptor_test.go`

- [ ] **Step 1: Write the failing tests**

Append to the end of `config/descriptor/descriptor_test.go`:

```go
func TestSectionShape_KeyedMapConstantDistinct(t *testing.T) {
	// ShapeKeyedMap must be distinct from both ShapeMap (zero-value default)
	// and ShapeScalar (added in Stage 4a). A collision would silently break
	// the schema DTO's shape-string emission.
	if ShapeKeyedMap == ShapeMap {
		t.Error("ShapeKeyedMap equals ShapeMap — they must be distinct")
	}
	if ShapeKeyedMap == ShapeScalar {
		t.Error("ShapeKeyedMap equals ShapeScalar — they must be distinct")
	}
}

func TestShapeKeyedMapInvariant_FlagsMissingProviderEnum(t *testing.T) {
	// Seed a ShapeKeyedMap section without the required provider-enum field
	// and verify the invariant logic would reject it.
	key := "__test_keyed_map_no_provider"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeKeyedMap,
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Base URL", Kind: FieldString},
		},
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key {
			continue
		}
		if s.Shape != ShapeKeyedMap {
			continue
		}
		var providerEnums int
		for _, f := range s.Fields {
			if f.Name == "provider" && f.Kind == FieldEnum {
				providerEnums++
			}
		}
		if providerEnums != 1 {
			fired = true
		}
	}
	if !fired {
		t.Error("invariant did not flag ShapeKeyedMap without a provider-enum field")
	}
}

func TestShapeKeyedMapInvariant_FlagsEmptyFields(t *testing.T) {
	key := "__test_keyed_map_empty_fields"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeKeyedMap,
		Fields:  nil,
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key {
			continue
		}
		if s.Shape == ShapeKeyedMap && len(s.Fields) == 0 {
			fired = true
		}
	}
	if !fired {
		t.Error("invariant did not flag ShapeKeyedMap with empty Fields")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./config/descriptor/... -run 'TestSectionShape_KeyedMapConstantDistinct|TestShapeKeyedMapInvariant'`

Expected: FAIL — compilation error on `undefined: ShapeKeyedMap`.

- [ ] **Step 3: Write minimal implementation**

Edit `config/descriptor/descriptor.go`. Locate the `SectionShape` const block (added in Stage 4a) and extend it:

```go
const (
	ShapeMap      SectionShape = iota
	ShapeScalar
	ShapeKeyedMap
)
```

Then extend `TestSectionInvariants` in `config/descriptor/descriptor_test.go`. Locate the outer loop `for _, s := range All() {`. After the existing ShapeScalar branch (`if s.Shape == ShapeScalar && len(s.Fields) != 1 { t.Errorf(...) }`), add:

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

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./config/descriptor/...`

Expected: PASS (all existing + new). Existing sections (storage, agent, terminal, logging, metrics, tracing, model, auxiliary) carry `Shape == ShapeMap` or `ShapeScalar` via the zero value or explicit setting from Stage 4a; none is ShapeKeyedMap, so the new invariant doesn't trigger on them.

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/descriptor.go config/descriptor/descriptor_test.go
git commit -m "feat(config/descriptor): ShapeKeyedMap for map-of-uniform-struct sections"
```

---

## Task 2: Schema DTO emits `"keyed_map"`

**Files:**
- Modify: `api/handlers_config_schema.go`
- Modify: `api/handlers_config_schema_test.go`

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_config_schema_test.go`:

```go
func TestConfigSchema_EmitsKeyedMapShapeString(t *testing.T) {
	// Seed a ShapeKeyedMap section directly via Register so this test doesn't
	// depend on Task 4's providers descriptor having landed.
	const key = "__test_schema_keyed_map"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeKeyedMap,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a", "b"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})
	t.Cleanup(func() {
		// There is no public Deregister; overwrite with a blank Section so
		// TestSectionInvariants across the package stays green.
		// The registry lives in the descriptor package and we can't touch
		// it from api_test. Instead, leave the seed in place; subsequent
		// tests filter by key.
	})

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
	var body struct {
		Sections []map[string]any `json:"sections"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found bool
	for _, sec := range body.Sections {
		if k, _ := sec["key"].(string); k == key {
			found = true
			if shape, _ := sec["shape"].(string); shape != "keyed_map" {
				t.Errorf("shape = %q, want %q", shape, "keyed_map")
			}
		}
	}
	if !found {
		t.Fatalf("seeded section %q not present in response", key)
	}
}
```

Note: because the descriptor registry is process-global and there is no `Deregister`, this seed sticks around for the duration of the test binary. That's acceptable: the key is `__test_schema_keyed_map` (prefix-isolated), and its 2 fields satisfy both invariants (1 FieldEnum named provider, len>0).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/... -run TestConfigSchema_EmitsKeyedMapShapeString`

Expected: FAIL — shape is `""` in the response because `shapeString` still has only the ShapeScalar branch.

- [ ] **Step 3: Write minimal implementation**

Edit `api/handlers_config_schema.go`. Replace the `shapeString` helper at the bottom of the file with:

```go
// shapeString converts a descriptor.SectionShape to the JSON-wire string.
// ShapeMap (zero value) returns "" so the DTO's omitempty tag drops the key.
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

- [ ] **Step 4: Run the full api test suite**

Run: `go test ./api/...`

Expected: PASS including `TestConfigSchema_EmitsKeyedMapShapeString`, `TestConfigSchema_OmitsShapeForMapSections` (Stage 4a tripwire), and `TestConfigSchema_IncludesStage4aSections`.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config_schema.go api/handlers_config_schema_test.go
git commit -m "feat(api): shapeString emits \"keyed_map\" for ShapeKeyedMap sections"
```

---

## Task 3: redact/preserve branches for ShapeKeyedMap

**Files:**
- Modify: `api/handlers_config.go`
- Modify: `api/handlers_config_test.go`

- [ ] **Step 1: Write the failing tests**

Append two tests to `api/handlers_config_test.go`:

```go
func TestConfigGet_RedactsKeyedMapSecrets(t *testing.T) {
	// Seed a ShapeKeyedMap section so we don't wait on Task 4's providers.
	const key = "__test_redact_keyed_map"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeKeyedMap,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})

	// The descriptor has yaml key __test_redact_keyed_map but config.Config has
	// no matching struct field. To simulate a populated instance, inject it
	// directly into the map that the GET handler receives. We do that by
	// passing a Config whose yaml-marshaled shape contains the key — easiest
	// is to bolt it onto config.Config.Providers since we're only exercising
	// the redaction walk, not the yaml round-trip. But Providers isn't
	// ShapeKeyedMap yet. Instead, seed the tested handler via a custom Config
	// struct — which we can't define here. Fall back to exercising redaction
	// through yaml.Marshal of a map[string]any we construct ourselves.
	// The redactSectionSecrets function operates on a map[string]any — we
	// call it directly.
	blob := map[string]any{
		key: map[string]any{
			"anthropic_main": map[string]any{
				"provider": "a",
				"api_key":  "sk-real-secret",
			},
			"openai_bot": map[string]any{
				"provider": "a",
				"api_key":  "sk-other-secret",
			},
		},
	}
	api.RedactSectionSecretsForTest(blob)
	inst1, _ := blob[key].(map[string]any)["anthropic_main"].(map[string]any)
	inst2, _ := blob[key].(map[string]any)["openai_bot"].(map[string]any)
	if inst1["api_key"] != "" {
		t.Errorf("anthropic_main.api_key = %q, want blank", inst1["api_key"])
	}
	if inst2["api_key"] != "" {
		t.Errorf("openai_bot.api_key = %q, want blank", inst2["api_key"])
	}
	if inst1["provider"] != "a" {
		t.Errorf("anthropic_main.provider = %q, want untouched", inst1["provider"])
	}
}

func TestConfigPut_PreservesKeyedMapSecrets(t *testing.T) {
	// Same seeding story. Exercise preserveSectionSecrets via a test hook.
	const key = "__test_preserve_keyed_map"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeKeyedMap,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})

	current := map[string]any{
		key: map[string]any{
			"anthropic_main": map[string]any{
				"provider": "a",
				"api_key":  "sk-real-secret",
			},
		},
	}
	updated := map[string]any{
		key: map[string]any{
			"anthropic_main": map[string]any{
				"provider": "a",
				"api_key":  "", // blanked — should be restored from current
			},
			"new_instance": map[string]any{
				"provider": "a",
				"api_key":  "sk-freshly-typed",
			},
		},
	}
	api.PreserveSectionSecretsForTest(updated, current)
	inst1, _ := updated[key].(map[string]any)["anthropic_main"].(map[string]any)
	if inst1["api_key"] != "sk-real-secret" {
		t.Errorf("anthropic_main.api_key = %q, want %q (restored from current)",
			inst1["api_key"], "sk-real-secret")
	}
	inst2, _ := updated[key].(map[string]any)["new_instance"].(map[string]any)
	if inst2["api_key"] != "sk-freshly-typed" {
		t.Errorf("new_instance.api_key = %q, want %q (preserved from updated)",
			inst2["api_key"], "sk-freshly-typed")
	}
}
```

Both tests reach into unexported helpers via test-only wrappers. Add these wrappers to `api/handlers_config.go` at the bottom of the file:

```go
// RedactSectionSecretsForTest is a test-only wrapper around redactSectionSecrets.
// Stage 4b tests use it to exercise the ShapeKeyedMap redaction path without
// wiring a full Config struct with a matching yaml field.
func RedactSectionSecretsForTest(m map[string]any) { redactSectionSecrets(m) }

// PreserveSectionSecretsForTest is a test-only wrapper around the
// preserveSectionSecrets inner logic that walks map[string]any instead of
// config.Config. It mimics what preserveSectionSecrets does after its
// YAML-marshal-and-unmarshal prelude. Matches the input shape the redact
// helper operates on.
func PreserveSectionSecretsForTest(updated, current map[string]any) {
	for _, sec := range descriptor.All() {
		if sec.Shape != descriptor.ShapeKeyedMap {
			continue
		}
		outer, ok := updated[sec.Key].(map[string]any)
		if !ok {
			continue
		}
		curOuter, _ := current[sec.Key].(map[string]any)
		for instKey, raw := range outer {
			inner, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			curInst, _ := curOuter[instKey].(map[string]any)
			for _, f := range sec.Fields {
				if f.Kind != descriptor.FieldSecret {
					continue
				}
				newVal, _ := inner[f.Name].(string)
				if newVal != "" {
					continue
				}
				if curInst == nil {
					continue
				}
				prevVal, _ := curInst[f.Name].(string)
				if prevVal == "" {
					continue
				}
				inner[f.Name] = prevVal
			}
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./api/... -run 'TestConfigGet_RedactsKeyedMapSecrets|TestConfigPut_PreservesKeyedMapSecrets'`

Expected: FAIL — `api/handlers_config.go` currently only handles `ShapeScalar` (continue) and `ShapeMap` (blob path). `redactSectionSecrets` doesn't iterate instances yet, so no api_keys get blanked.

- [ ] **Step 3: Write minimal implementation**

Edit `api/handlers_config.go`. Replace the `redactSectionSecrets` body with:

```go
func redactSectionSecrets(m map[string]any) {
	for _, sec := range descriptor.All() {
		if sec.Shape == descriptor.ShapeScalar {
			// Scalar sections have no nested map. 4a has no scalar secrets.
			continue
		}
		if sec.Shape == descriptor.ShapeKeyedMap {
			// Walk map[string]any of instances, each itself map[string]any.
			outer, ok := m[sec.Key].(map[string]any)
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
	}
}
```

Then edit `preserveSectionSecrets`. Find the loop `for _, sec := range sections` (after the YAML marshal/unmarshal prelude) and replace its body so it branches on `Shape`:

```go
	changed := false
	for _, sec := range sections {
		if sec.Shape == descriptor.ShapeScalar {
			continue
		}
		if sec.Shape == descriptor.ShapeKeyedMap {
			outer, ok := updM[sec.Key].(map[string]any)
			if !ok {
				continue
			}
			curOuter, _ := curM[sec.Key].(map[string]any)
			for instKey, raw := range outer {
				inner, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				curInst, _ := curOuter[instKey].(map[string]any)
				for _, f := range sec.Fields {
					if f.Kind != descriptor.FieldSecret {
						continue
					}
					newVal, _ := inner[f.Name].(string)
					if newVal != "" {
						continue
					}
					if curInst == nil {
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
	}
```

Keep the rest of `preserveSectionSecrets` (the marshal/unmarshal prelude and the re-marshal at the end) exactly as it was.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./api/...`

Expected: PASS (all including the two new tests + the Stage 4a tripwires).

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go
git commit -m "feat(api): redact/preserve ShapeKeyedMap instances per secret field"
```

---

## Task 4: `providers` descriptor

**Files:**
- Create: `config/descriptor/providers.go`
- Create: `config/descriptor/providers_test.go`

Depends on Task 1 (ShapeKeyedMap constant) and the existing `factory.Types()` from Stage 4a Task 2.

- [ ] **Step 1: Write the failing test**

Create `config/descriptor/providers_test.go`:

```go
package descriptor

import "testing"

func TestProvidersSectionRegistered(t *testing.T) {
	s, ok := Get("providers")
	if !ok {
		t.Fatal(`Get("providers") returned ok=false — did providers.go init() register?`)
	}
	if s.GroupID != "models" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "models")
	}
	if s.Shape != ShapeKeyedMap {
		t.Errorf("Shape = %v, want ShapeKeyedMap", s.Shape)
	}

	want := map[string]FieldKind{
		"provider": FieldEnum,
		"base_url": FieldString,
		"api_key":  FieldSecret,
		"model":    FieldString,
	}
	got := map[string]FieldKind{}
	for _, f := range s.Fields {
		got[f.Name] = f.Kind
	}
	for name, kind := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if g != kind {
			t.Errorf("field %q: Kind = %s, want %s", name, g, kind)
		}
	}

	// Required flags — provider and api_key are required; base_url and model optional.
	for _, f := range s.Fields {
		switch f.Name {
		case "provider", "api_key":
			if !f.Required {
				t.Errorf("field %q: Required = false, want true", f.Name)
			}
		case "base_url", "model":
			if f.Required {
				t.Errorf("field %q: Required = true, want false", f.Name)
			}
		}
	}
}

func TestProvidersProviderEnumPopulatedFromFactory(t *testing.T) {
	s, _ := Get("providers")
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
	if len(provider.Enum) == 0 {
		t.Fatal("provider.Enum is empty — did providers.go import provider/factory correctly?")
	}
	has := map[string]bool{}
	for _, v := range provider.Enum {
		has[v] = true
	}
	for _, want := range []string{"anthropic", "openai"} {
		if !has[want] {
			t.Errorf("provider.Enum missing %q; got %v", want, provider.Enum)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/descriptor/... -run TestProviders`

Expected: FAIL with `Get("providers") returned ok=false`.

- [ ] **Step 3: Write minimal implementation**

Create `config/descriptor/providers.go`:

```go
package descriptor

import "github.com/odysseythink/hermind/provider/factory"

// Providers mirrors config.Config.Providers (map[string]config.ProviderConfig).
// Each instance conforms to the same 4-field schema regardless of provider
// type — unlike gateway.platforms where each type has distinct fields.
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
				Label: "Default model for this provider",
				Help:  "Optional — provider-qualified id used when a request doesn't pin a specific model.",
				Kind:  FieldString,
			},
		},
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./config/descriptor/...`

Expected: PASS (including `TestSectionInvariants` which validates ShapeKeyedMap requires exactly one provider-enum and len(Fields) > 0 — the providers descriptor satisfies both).

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/providers.go config/descriptor/providers_test.go
git commit -m "feat(config/descriptor): Providers section (ShapeKeyedMap, 4-field schema)"
```

---

## Task 5: `POST /api/providers/{name}/models` endpoint

**Files:**
- Create: `api/handlers_providers_models.go`
- Create: `api/handlers_providers_models_test.go`
- Modify: `api/server.go`

- [ ] **Step 1: Write the failing tests**

Create `api/handlers_providers_models_test.go`:

```go
package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	_ "github.com/odysseythink/hermind/config/descriptor"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// fakeProvider implements provider.Provider with optional ModelLister behavior.
// Injected into factory.primary via the test hook below.
type fakeProvider struct {
	models []string
	err    error
	lister bool
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Chat(ctx context.Context, _ []provider.Message, _ provider.ChatOptions) (provider.ChatResult, error) {
	return provider.ChatResult{}, errors.New("not implemented")
}
func (f *fakeProvider) ChatStream(ctx context.Context, _ []provider.Message, _ provider.ChatOptions, _ provider.StreamSink) error {
	return errors.New("not implemented")
}
func (f *fakeProvider) Metadata() provider.Metadata { return provider.Metadata{} }
func (f *fakeProvider) ListModels(ctx context.Context) ([]string, error) {
	return f.models, f.err
}

// fakeProviderNoLister returns a provider that does NOT implement ModelLister.
type fakeProviderNoLister struct{}

func (f *fakeProviderNoLister) Name() string { return "fake-no-lister" }
func (f *fakeProviderNoLister) Chat(ctx context.Context, _ []provider.Message, _ provider.ChatOptions) (provider.ChatResult, error) {
	return provider.ChatResult{}, errors.New("not implemented")
}
func (f *fakeProviderNoLister) ChatStream(ctx context.Context, _ []provider.Message, _ provider.ChatOptions, _ provider.StreamSink) error {
	return errors.New("not implemented")
}
func (f *fakeProviderNoLister) Metadata() provider.Metadata { return provider.Metadata{} }

func TestProvidersModels_HappyPath(t *testing.T) {
	factory.SetConstructorForTest("__fake_happy", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProvider{models: []string{"m1", "m2"}}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_happy") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_happy", APIKey: "k"},
	}}
	srv, err := api.NewServer(&api.ServerOpts{Config: cfg, Token: "t"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(body.Models) != 2 || body.Models[0] != "m1" || body.Models[1] != "m2" {
		t.Errorf("models = %v, want [m1 m2]", body.Models)
	}
}

func TestProvidersModels_UnknownName(t *testing.T) {
	cfg := &config.Config{Providers: map[string]config.ProviderConfig{}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "t"})
	req := httptest.NewRequest("POST", "/api/providers/does-not-exist/models", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestProvidersModels_FactoryError(t *testing.T) {
	factory.SetConstructorForTest("__fake_factory_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return nil, errors.New("bad config")
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_factory_err") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_factory_err"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "t"})
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestProvidersModels_NotListModelsCapable(t *testing.T) {
	factory.SetConstructorForTest("__fake_no_lister", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProviderNoLister{}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_no_lister") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_no_lister"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "t"})
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", w.Code)
	}
}

func TestProvidersModels_UpstreamError(t *testing.T) {
	factory.SetConstructorForTest("__fake_upstream_err", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &fakeProvider{err: errors.New("auth failed")}, nil
	})
	t.Cleanup(func() { factory.ClearConstructorForTest("__fake_upstream_err") })

	cfg := &config.Config{Providers: map[string]config.ProviderConfig{
		"p1": {Provider: "__fake_upstream_err"},
	}}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "t"})
	req := httptest.NewRequest("POST", "/api/providers/p1/models", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
```

The tests reach `factory.SetConstructorForTest` / `ClearConstructorForTest`. Add these hooks to `provider/factory/types.go` (not `factory.go` — Task 2 from Stage 4a owns that file and we want to keep `primary` private):

```go
// SetConstructorForTest injects a constructor under the given name. Test-only.
// Calls must be paired with ClearConstructorForTest to avoid cross-test pollution.
func SetConstructorForTest(name string, ctor func(cfg config.ProviderConfig) (provider.Provider, error)) {
	primary[name] = ctor
}

// ClearConstructorForTest removes a test-injected constructor.
func ClearConstructorForTest(name string) {
	delete(primary, name)
}
```

This requires `config` and `provider` imports in `types.go` (currently it only has `sort`). Update the import block:

```go
package factory

import (
	"sort"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./api/... -run TestProvidersModels`

Expected: FAIL — either compile error (route doesn't exist) or 404 on every case (because the handler isn't wired).

- [ ] **Step 3: Write minimal implementation**

Create `api/handlers_providers_models.go`:

```go
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/factory"
)

// handleProvidersModels responds to POST /api/providers/{name}/models.
// Reads config.Providers[name], dispatches via factory.New, type-asserts
// provider.ModelLister, and calls ListModels with a 10s timeout.
// Matches the legacy cli/ui/webconfig/handlers.go:258 behavior but lives
// in the Stage-1+ api package.
//
// Status codes:
//   200 - {"models": ["id", ...]}
//   400 - factory.New rejected the stored config
//   404 - no Providers[name] in stored config
//   501 - provider type exists but its constructor doesn't implement ModelLister
//   502 - upstream provider errored (network, auth, rate-limit, ...)
func (s *Server) handleProvidersModels(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg, ok := s.opts.Config.Providers[name]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown provider %q", name), http.StatusNotFound)
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

Edit `api/server.go` to wire the route. Find the block where routes are registered under `r.Route("/api", …)` (around line 128) and add after the existing `r.Get("/providers", s.handleProvidersList)` line:

```go
		r.Post("/providers/{name}/models", s.handleProvidersModels)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./api/... ./provider/factory/...`

Expected: PASS for all 5 new cases plus the existing suites.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_providers_models.go api/handlers_providers_models_test.go api/server.go provider/factory/types.go
git commit -m "feat(api): POST /api/providers/{name}/models fetch + connection test"
```

---

## Task 6: Frontend Zod schema extension

**Files:**
- Modify: `web/src/api/schemas.ts`
- Modify: `web/src/api/schemas.test.ts`

- [ ] **Step 1: Write the failing tests**

Append to `web/src/api/schemas.test.ts`:

```ts
describe('ConfigSectionSchema — keyed_map shape', () => {
  it('accepts shape: "keyed_map"', () => {
    const parsed = ConfigSectionSchema.parse({
      key: 'providers',
      label: 'Providers',
      group_id: 'models',
      shape: 'keyed_map',
      fields: [
        { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
          enum: ['anthropic', 'openai'] },
        { name: 'api_key', label: 'API key', kind: 'secret', required: true },
      ],
    });
    expect(parsed.shape).toBe('keyed_map');
  });
});

describe('ProviderModelsResponseSchema', () => {
  it('accepts a valid models list', () => {
    const parsed = ProviderModelsResponseSchema.parse({
      models: ['claude-opus-4-7', 'claude-sonnet-4-6'],
    });
    expect(parsed.models).toEqual(['claude-opus-4-7', 'claude-sonnet-4-6']);
  });

  it('accepts an empty models list', () => {
    const parsed = ProviderModelsResponseSchema.parse({ models: [] });
    expect(parsed.models).toEqual([]);
  });

  it('rejects missing models key', () => {
    expect(() => ProviderModelsResponseSchema.parse({})).toThrow();
  });

  it('rejects non-string model entries', () => {
    expect(() =>
      ProviderModelsResponseSchema.parse({ models: [1, 2] }),
    ).toThrow();
  });
});
```

And add `ProviderModelsResponseSchema` to the imports at the top of `schemas.test.ts` (alphabetical insertion into the existing `{...}` block imported from `./schemas`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && pnpm vitest run src/api/schemas.test.ts`

Expected: FAIL on `keyed_map` (not in the enum) and on `ProviderModelsResponseSchema` (undefined export).

- [ ] **Step 3: Write minimal implementation**

Edit `web/src/api/schemas.ts`. Update the `shape` enum inside `ConfigSectionSchema`:

```ts
export const ConfigSectionSchema = z.object({
  key: z.string(),
  label: z.string(),
  summary: z.string().optional(),
  group_id: z.string(),
  shape: z.enum(['map', 'scalar', 'keyed_map']).optional(), // default (absent) = map
  fields: z.array(ConfigFieldSchema),
});
```

Then append at the end of the file:

```ts
export const ProviderModelsResponseSchema = z.object({
  models: z.array(z.string()),
});
export type ProviderModelsResponse = z.infer<typeof ProviderModelsResponseSchema>;
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && pnpm vitest run src/api/schemas.test.ts`

Expected: PASS including all Stage 4a schema tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/api/schemas.ts web/src/api/schemas.test.ts
git commit -m "feat(web): Zod schemas for keyed_map shape + ProviderModels response"
```

---

## Task 7: State reducer — `keyed-instance/*` actions

**Files:**
- Modify: `web/src/state.ts`
- Modify: `web/src/state.test.ts`

- [ ] **Step 1: Write the failing tests**

Append to `web/src/state.test.ts`:

```ts
describe('reducer: edit/keyed-instance-field', () => {
  it('sets a field on an existing instance', () => {
    const s = {
      ...initialState,
      config: {
        providers: {
          anthropic_main: { provider: 'anthropic', base_url: '', api_key: '', model: '' },
        },
      } as any,
    };
    const next = reducer(s, {
      type: 'edit/keyed-instance-field',
      sectionKey: 'providers',
      instanceKey: 'anthropic_main',
      field: 'base_url',
      value: 'https://api.anthropic.com',
    });
    const p = (next.config as any).providers.anthropic_main;
    expect(p.base_url).toBe('https://api.anthropic.com');
    expect(p.provider).toBe('anthropic'); // unchanged
  });

  it('does not touch other instances', () => {
    const s = {
      ...initialState,
      config: {
        providers: {
          a: { provider: 'x', model: '' },
          b: { provider: 'y', model: 'other' },
        },
      } as any,
    };
    const next = reducer(s, {
      type: 'edit/keyed-instance-field',
      sectionKey: 'providers',
      instanceKey: 'a',
      field: 'model',
      value: 'anthropic/claude-opus-4-7',
    });
    expect((next.config as any).providers.b).toEqual({ provider: 'y', model: 'other' });
  });
});

describe('reducer: keyed-instance/create', () => {
  it('adds a new instance with the initial payload', () => {
    const s = {
      ...initialState,
      config: { providers: {} } as any,
    };
    const next = reducer(s, {
      type: 'keyed-instance/create',
      sectionKey: 'providers',
      instanceKey: 'openai_bot',
      initial: { provider: 'openai', base_url: '', api_key: '', model: '' },
    });
    expect((next.config as any).providers.openai_bot).toEqual({
      provider: 'openai', base_url: '', api_key: '', model: '',
    });
  });

  it('creates the section map if it did not exist', () => {
    const s = { ...initialState, config: {} as any };
    const next = reducer(s, {
      type: 'keyed-instance/create',
      sectionKey: 'providers',
      instanceKey: 'p1',
      initial: { provider: 'openai' },
    });
    expect((next.config as any).providers.p1.provider).toBe('openai');
  });
});

describe('reducer: keyed-instance/delete', () => {
  it('removes an existing instance', () => {
    const s = {
      ...initialState,
      config: {
        providers: { a: { provider: 'x' }, b: { provider: 'y' } },
      } as any,
    };
    const next = reducer(s, {
      type: 'keyed-instance/delete',
      sectionKey: 'providers',
      instanceKey: 'a',
    });
    expect((next.config as any).providers.a).toBeUndefined();
    expect((next.config as any).providers.b).toEqual({ provider: 'y' });
  });

  it('is a no-op for missing keys', () => {
    const s = {
      ...initialState,
      config: { providers: { a: { provider: 'x' } } } as any,
    };
    const next = reducer(s, {
      type: 'keyed-instance/delete',
      sectionKey: 'providers',
      instanceKey: 'nonexistent',
    });
    expect((next.config as any).providers.a).toEqual({ provider: 'x' });
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && pnpm vitest run src/state.test.ts`

Expected: FAIL — the three action types are not in the Action union so the dispatch call compile-errors.

- [ ] **Step 3: Write minimal implementation**

Edit `web/src/state.ts`. Extend the `Action` union (currently ends with `'edit/config-scalar'` from Stage 4a):

```ts
export type Action =
  | { type: 'boot/loaded'; descriptors: SchemaDescriptor[]; configSections: ConfigSection[]; config: Config }
  | { type: 'boot/failed'; error: string }
  | { type: 'select'; key: string | null }
  | { type: 'flash'; flash: Flash | null }
  | { type: 'save/start' }
  | { type: 'save/done'; error?: string }
  | { type: 'apply/start' }
  | { type: 'apply/done'; error?: string }
  | { type: 'edit/field'; key: string; field: string; value: string }
  | { type: 'edit/enabled'; key: string; enabled: boolean }
  | { type: 'instance/delete'; key: string }
  | { type: 'instance/create'; key: string; platformType: string }
  | { type: 'shell/selectGroup'; group: GroupId | null }
  | { type: 'shell/selectSub'; key: string | null }
  | { type: 'shell/toggleGroup'; group: GroupId }
  | { type: 'edit/config-field'; sectionKey: string; field: string; value: unknown }
  | { type: 'edit/config-scalar'; sectionKey: string; value: unknown }
  | { type: 'edit/keyed-instance-field'; sectionKey: string; instanceKey: string; field: string; value: unknown }
  | { type: 'keyed-instance/create'; sectionKey: string; instanceKey: string; initial: Record<string, unknown> }
  | { type: 'keyed-instance/delete'; sectionKey: string; instanceKey: string };
```

In the `reducer` function, add three new cases after `edit/config-scalar`:

```ts
    case 'edit/keyed-instance-field': {
      const cfg = state.config as unknown as Record<string, unknown>;
      const sec = (cfg[action.sectionKey] as Record<string, Record<string, unknown>> | undefined) ?? {};
      const inst = sec[action.instanceKey] ?? {};
      return {
        ...state,
        config: {
          ...state.config,
          [action.sectionKey]: {
            ...sec,
            [action.instanceKey]: { ...inst, [action.field]: action.value },
          },
        } as typeof state.config,
      };
    }
    case 'keyed-instance/create': {
      const cfg = state.config as unknown as Record<string, unknown>;
      const sec = (cfg[action.sectionKey] as Record<string, Record<string, unknown>> | undefined) ?? {};
      return {
        ...state,
        config: {
          ...state.config,
          [action.sectionKey]: {
            ...sec,
            [action.instanceKey]: action.initial,
          },
        } as typeof state.config,
      };
    }
    case 'keyed-instance/delete': {
      const cfg = state.config as unknown as Record<string, unknown>;
      const sec = (cfg[action.sectionKey] as Record<string, Record<string, unknown>> | undefined) ?? {};
      if (!(action.instanceKey in sec)) {
        return state;
      }
      const next = { ...sec };
      delete next[action.instanceKey];
      return {
        ...state,
        config: {
          ...state.config,
          [action.sectionKey]: next,
        } as typeof state.config,
      };
    }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && pnpm vitest run src/state.test.ts`

Expected: PASS — all three describe blocks green (7 new assertions total across them).

- [ ] **Step 5: Commit**

```bash
git add web/src/state.ts web/src/state.test.ts
git commit -m "feat(web/state): keyed-instance actions (create, delete, edit-field)"
```

---

## Task 8: `keyedInstanceDirty` selector

**Files:**
- Create: `web/src/shell/keyedInstances.ts`
- Create: `web/src/shell/keyedInstances.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/shell/keyedInstances.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { keyedInstanceDirty } from './keyedInstances';
import { initialState, type AppState } from '../state';

function stateWith(current: any, original: any): AppState {
  return { ...initialState, config: current, originalConfig: original } as AppState;
}

describe('keyedInstanceDirty', () => {
  it('returns false when instance is identical between current and original', () => {
    const inst = { provider: 'anthropic', base_url: '', api_key: '', model: '' };
    const s = stateWith(
      { providers: { a: inst } },
      { providers: { a: inst } },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(false);
  });

  it('returns true when a field diverges', () => {
    const s = stateWith(
      { providers: { a: { provider: 'anthropic', base_url: 'https://edited' } } },
      { providers: { a: { provider: 'anthropic', base_url: '' } } },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(true);
  });

  it('returns true for newly created instances (absent in original)', () => {
    const s = stateWith(
      { providers: { a: { provider: 'anthropic' } } },
      { providers: {} },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(true);
  });

  it('returns true for deleted instances (absent in current)', () => {
    const s = stateWith(
      { providers: {} },
      { providers: { a: { provider: 'anthropic' } } },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(true);
  });

  it('returns false when both current and original lack the instance', () => {
    const s = stateWith({ providers: {} }, { providers: {} });
    expect(keyedInstanceDirty(s, 'providers', 'absent')).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && pnpm vitest run src/shell/keyedInstances.test.ts`

Expected: FAIL — file doesn't exist yet, import error.

- [ ] **Step 3: Write minimal implementation**

Create `web/src/shell/keyedInstances.ts`:

```ts
import type { AppState } from '../state';

/**
 * Returns true when a ShapeKeyedMap instance differs between the current
 * and original config slices. Used by ModelsSidebar to render a per-instance
 * dirty dot. An instance is dirty if it is present-only-in-one-side or if
 * any field value diverges.
 */
export function keyedInstanceDirty(
  state: AppState,
  sectionKey: string,
  instanceKey: string,
): boolean {
  const cur = readInstance(state.config, sectionKey, instanceKey);
  const orig = readInstance(state.originalConfig, sectionKey, instanceKey);
  if (cur === undefined && orig === undefined) return false;
  if (cur === undefined || orig === undefined) return true;
  return !shallowEqual(cur, orig);
}

function readInstance(
  cfg: unknown,
  sectionKey: string,
  instanceKey: string,
): Record<string, unknown> | undefined {
  const root = cfg as Record<string, unknown> | null | undefined;
  if (!root) return undefined;
  const sec = root[sectionKey] as Record<string, unknown> | undefined;
  if (!sec) return undefined;
  return sec[instanceKey] as Record<string, unknown> | undefined;
}

function shallowEqual(a: Record<string, unknown>, b: Record<string, unknown>): boolean {
  const ak = Object.keys(a);
  const bk = Object.keys(b);
  if (ak.length !== bk.length) return false;
  for (const k of ak) {
    if (a[k] !== b[k]) return false;
  }
  return true;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && pnpm vitest run src/shell/keyedInstances.test.ts`

Expected: PASS on all 5 cases.

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/keyedInstances.ts web/src/shell/keyedInstances.test.ts
git commit -m "feat(web/shell): keyedInstanceDirty selector for ShapeKeyedMap sections"
```

---

## Task 9: Three Models-group components

**Files:**
- Create: `web/src/components/groups/models/ModelsSidebar.tsx`
- Create: `web/src/components/groups/models/ModelsSidebar.module.css`
- Create: `web/src/components/groups/models/ModelsSidebar.test.tsx`
- Create: `web/src/components/groups/models/ProviderEditor.tsx`
- Create: `web/src/components/groups/models/ProviderEditor.module.css`
- Create: `web/src/components/groups/models/ProviderEditor.test.tsx`
- Create: `web/src/components/groups/models/NewProviderDialog.tsx`
- Create: `web/src/components/groups/models/NewProviderDialog.module.css`
- Create: `web/src/components/groups/models/NewProviderDialog.test.tsx`

- [ ] **Step 1: Write failing tests**

Create `web/src/components/groups/models/ModelsSidebar.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ModelsSidebar from './ModelsSidebar';

describe('ModelsSidebar', () => {
  const baseProps = {
    instances: [
      { key: 'anthropic_main', type: 'anthropic' },
      { key: 'openai_bot', type: 'openai' },
    ],
    activeSubKey: null as string | null,
    dirtyKeys: new Set<string>(),
    onSelectScalar: () => {},
    onSelectInstance: () => {},
    onNewProvider: () => {},
  };

  it('renders the Default Model scalar row plus one row per instance', () => {
    render(<ModelsSidebar {...baseProps} />);
    expect(screen.getByText('Default model')).toBeInTheDocument();
    expect(screen.getByText('anthropic_main')).toBeInTheDocument();
    expect(screen.getByText('openai_bot')).toBeInTheDocument();
  });

  it('fires onSelectScalar("model") when Default model is clicked', async () => {
    const user = userEvent.setup();
    const onSelectScalar = vi.fn();
    render(<ModelsSidebar {...baseProps} onSelectScalar={onSelectScalar} />);
    await user.click(screen.getByText('Default model'));
    expect(onSelectScalar).toHaveBeenCalledWith('model');
  });

  it('fires onSelectInstance(key) when an instance row is clicked', async () => {
    const user = userEvent.setup();
    const onSelectInstance = vi.fn();
    render(<ModelsSidebar {...baseProps} onSelectInstance={onSelectInstance} />);
    await user.click(screen.getByText('anthropic_main'));
    expect(onSelectInstance).toHaveBeenCalledWith('anthropic_main');
  });

  it('fires onNewProvider() when the + button is clicked', async () => {
    const user = userEvent.setup();
    const onNewProvider = vi.fn();
    render(<ModelsSidebar {...baseProps} onNewProvider={onNewProvider} />);
    await user.click(screen.getByRole('button', { name: /new provider/i }));
    expect(onNewProvider).toHaveBeenCalled();
  });

  it('shows a dirty dot on rows whose key is in dirtyKeys', () => {
    render(
      <ModelsSidebar
        {...baseProps}
        dirtyKeys={new Set(['openai_bot'])}
      />,
    );
    const dirtyRow = screen.getByText('openai_bot').closest('button');
    expect(dirtyRow?.querySelector('[title="Unsaved changes"]')).toBeTruthy();
  });
});
```

Create `web/src/components/groups/models/NewProviderDialog.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import NewProviderDialog from './NewProviderDialog';

describe('NewProviderDialog', () => {
  const baseProps = {
    providerTypes: ['anthropic', 'openai', 'openrouter'],
    existingKeys: new Set<string>(['already_taken']),
    onCancel: () => {},
    onCreate: () => {},
  };

  it('renders the type dropdown populated from providerTypes', () => {
    render(<NewProviderDialog {...baseProps} />);
    const select = screen.getByLabelText(/provider type/i) as HTMLSelectElement;
    const options = Array.from(select.querySelectorAll('option')).map(o => o.value);
    expect(options).toEqual(['anthropic', 'openai', 'openrouter']);
  });

  it('rejects malformed keys', async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<NewProviderDialog {...baseProps} onCreate={onCreate} />);
    await user.type(screen.getByLabelText(/instance key/i), 'BAD KEY');
    await user.click(screen.getByRole('button', { name: /create/i }));
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByText(/lowercase letters, digits, underscore/i)).toBeInTheDocument();
  });

  it('rejects duplicate keys', async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<NewProviderDialog {...baseProps} onCreate={onCreate} />);
    await user.type(screen.getByLabelText(/instance key/i), 'already_taken');
    await user.click(screen.getByRole('button', { name: /create/i }));
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByText(/already exists/i)).toBeInTheDocument();
  });

  it('dispatches onCreate(key, type) for valid input', async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(<NewProviderDialog {...baseProps} onCreate={onCreate} />);
    await user.type(screen.getByLabelText(/instance key/i), 'openai_bot');
    await user.selectOptions(screen.getByLabelText(/provider type/i), 'openai');
    await user.click(screen.getByRole('button', { name: /create/i }));
    expect(onCreate).toHaveBeenCalledWith('openai_bot', 'openai');
  });

  it('fires onCancel when the Cancel button is clicked', async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(<NewProviderDialog {...baseProps} onCancel={onCancel} />);
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalled();
  });
});
```

Create `web/src/components/groups/models/ProviderEditor.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ProviderEditor from './ProviderEditor';
import type { ConfigSection } from '../../../api/schemas';

const providersSection: ConfigSection = {
  key: 'providers',
  label: 'Providers',
  group_id: 'models',
  shape: 'keyed_map',
  fields: [
    { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
      enum: ['anthropic', 'openai'] },
    { name: 'base_url', label: 'Base URL', kind: 'string' },
    { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    { name: 'model', label: 'Default model for this provider', kind: 'string' },
  ],
};

const inst = {
  provider: 'anthropic',
  base_url: 'https://api.anthropic.com',
  api_key: '',
  model: 'anthropic/claude-opus-4-7',
};

describe('ProviderEditor', () => {
  function baseProps(overrides: any = {}) {
    return {
      sectionKey: 'providers',
      instanceKey: 'anthropic_main',
      section: providersSection,
      value: inst,
      originalValue: inst,
      dirty: false,
      onField: () => {},
      onDelete: () => {},
      fetchModels: async () => ({ models: [] }),
      ...overrides,
    };
  }

  it('renders all four fields', () => {
    render(<ProviderEditor {...baseProps()} />);
    expect(screen.getByLabelText(/provider type/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/base url/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/default model for this provider/i)).toBeInTheDocument();
  });

  it('dispatches onField with the correct instance key when a field changes', async () => {
    const user = userEvent.setup();
    const onField = vi.fn();
    render(<ProviderEditor {...baseProps({ onField })} />);
    const input = screen.getByLabelText(/base url/i);
    await user.clear(input);
    await user.type(input, 'https://new');
    const calls = onField.mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    const last = calls[calls.length - 1];
    expect(last[0]).toBe('anthropic_main');
    expect(last[1]).toBe('base_url');
    expect(last[2]).toBe('https://new');
  });

  it('disables the Fetch models button when the instance is dirty', () => {
    render(<ProviderEditor {...baseProps({ dirty: true })} />);
    const btn = screen.getByRole('button', { name: /fetch models/i });
    expect(btn).toBeDisabled();
  });

  it('populates the datalist from a successful fetch', async () => {
    const user = userEvent.setup();
    const fetchModels = vi.fn().mockResolvedValue({ models: ['claude-opus-4-7', 'claude-sonnet-4-6'] });
    render(<ProviderEditor {...baseProps({ fetchModels })} />);
    await user.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      expect(screen.getByText(/connected/i)).toBeInTheDocument();
    });
    const datalist = screen.getByTestId('provider-model-datalist');
    const options = Array.from(datalist.querySelectorAll('option')).map(o => o.getAttribute('value'));
    expect(options).toEqual(['claude-opus-4-7', 'claude-sonnet-4-6']);
  });

  it('shows an error chip when fetchModels rejects', async () => {
    const user = userEvent.setup();
    const fetchModels = vi.fn().mockRejectedValue(new Error('401 unauthorized'));
    render(<ProviderEditor {...baseProps({ fetchModels })} />);
    await user.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      expect(screen.getByText(/401 unauthorized/i)).toBeInTheDocument();
    });
  });

  it('prompts confirm and fires onDelete when Delete is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<ProviderEditor {...baseProps({ onDelete })} />);
    await user.click(screen.getByRole('button', { name: /delete/i }));
    expect(confirmSpy).toHaveBeenCalled();
    expect(onDelete).toHaveBeenCalled();
    confirmSpy.mockRestore();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && pnpm vitest run src/components/groups/models/`

Expected: FAIL — none of the three component files exist yet.

- [ ] **Step 3: Write the components**

Create `web/src/components/groups/models/ModelsSidebar.tsx`:

```tsx
import styles from './ModelsSidebar.module.css';

export interface ModelsSidebarProps {
  instances: Array<{ key: string; type: string }>;
  activeSubKey: string | null;
  dirtyKeys: Set<string>;
  onSelectScalar: (key: string) => void;
  onSelectInstance: (key: string) => void;
  onNewProvider: () => void;
}

export default function ModelsSidebar({
  instances,
  activeSubKey,
  dirtyKeys,
  onSelectScalar,
  onSelectInstance,
  onNewProvider,
}: ModelsSidebarProps) {
  return (
    <div className={styles.sidebar}>
      <button
        type="button"
        className={`${styles.scalarRow} ${activeSubKey === 'model' ? styles.active : ''}`}
        onClick={() => onSelectScalar('model')}
      >
        Default model
      </button>
      <div className={styles.groupHeader}>Providers</div>
      {instances.length === 0 && (
        <div className={styles.empty}>No providers configured.</div>
      )}
      {instances.map(inst => (
        <button
          key={inst.key}
          type="button"
          className={`${styles.item} ${inst.key === activeSubKey ? styles.active : ''}`}
          onClick={() => onSelectInstance(inst.key)}
        >
          <span className={styles.itemRow}>
            <span className={styles.itemKey}>{inst.key}</span>
            {dirtyKeys.has(inst.key) && (
              <span className={styles.dirtyDot} title="Unsaved changes" />
            )}
          </span>
          <span className={styles.itemType}>{inst.type}</span>
        </button>
      ))}
      <button type="button" className={styles.newBtn} onClick={onNewProvider}>
        + New provider
      </button>
    </div>
  );
}
```

Create `web/src/components/groups/models/ModelsSidebar.module.css`:

```css
.sidebar {
  padding: 4px 0 8px;
}

.scalarRow, .item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 6px 12px 6px 24px;
  background: transparent;
  border: 0;
  color: var(--fg, #c9d1d9);
  font-size: 13px;
  cursor: pointer;
}

.scalarRow:hover, .item:hover {
  background: var(--bg-hover, #1b1f27);
}

.active {
  background: var(--bg-active, #1f6feb33);
  color: var(--fg-strong, #f0f6fc);
}

.groupHeader {
  padding: 10px 12px 4px 24px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--fg-muted, #8b949e);
}

.empty {
  padding: 4px 12px 4px 24px;
  font-size: 12px;
  font-style: italic;
  color: var(--fg-muted, #8b949e);
}

.itemRow {
  display: flex;
  align-items: center;
  gap: 6px;
}

.itemKey { flex: 1; }

.dirtyDot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--warn, #d29922);
}

.itemType {
  display: block;
  font-size: 11px;
  color: var(--fg-muted, #8b949e);
}

.newBtn {
  display: block;
  width: calc(100% - 24px);
  margin: 8px 12px 4px 24px;
  padding: 6px 10px;
  background: transparent;
  border: 1px dashed var(--border, #30363d);
  border-radius: 3px;
  color: var(--fg-muted, #8b949e);
  font-size: 12px;
  cursor: pointer;
}

.newBtn:hover {
  border-color: var(--fg-muted, #8b949e);
  color: var(--fg, #c9d1d9);
}
```

Create `web/src/components/groups/models/ProviderEditor.tsx`:

```tsx
import { useEffect, useId, useRef, useState } from 'react';
import styles from './ProviderEditor.module.css';
import ConfigSection from '../../ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../../../api/schemas';

export interface ProviderEditorProps {
  sectionKey: string;
  instanceKey: string;
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  dirty: boolean;
  onField: (instanceKey: string, field: string, value: unknown) => void;
  onDelete: () => void;
  fetchModels: () => Promise<{ models: string[] }>;
}

type FetchState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; count: number }
  | { status: 'err'; error: string };

export default function ProviderEditor(props: ProviderEditorProps) {
  const [models, setModels] = useState<string[]>([]);
  const [fetchState, setFetchState] = useState<FetchState>({ status: 'idle' });
  const datalistId = useId();
  const bodyRef = useRef<HTMLDivElement>(null);

  // Wire the Model field's <input> to the sibling <datalist> so the browser's
  // native autocomplete uses the fetched model list. ConfigSection renders
  // TextInput without a `list` attribute; rather than invading ConfigSection's
  // prop shape for this one case we set it from the outside after every render.
  useEffect(() => {
    const modelField = props.section.fields.find(f => f.name === 'model');
    if (!modelField || !bodyRef.current) return;
    const input = bodyRef.current.querySelector<HTMLInputElement>(
      `input[aria-label="${modelField.label}"]`,
    );
    if (input) input.setAttribute('list', datalistId);
  });

  async function onFetchClick() {
    setFetchState({ status: 'loading' });
    try {
      const { models: got } = await props.fetchModels();
      setModels(got);
      setFetchState({ status: 'ok', count: got.length });
    } catch (err) {
      setFetchState({ status: 'err', error: err instanceof Error ? err.message : String(err) });
    }
  }

  function onDeleteClick() {
    if (window.confirm(`Delete provider "${props.instanceKey}"? This cannot be undone.`)) {
      props.onDelete();
    }
  }

  return (
    <section className={styles.editor}>
      <header className={styles.header}>
        <div className={styles.breadcrumb}>
          Models / Providers / <strong>{props.instanceKey}</strong>
        </div>
        <button type="button" className={styles.deleteBtn} onClick={onDeleteClick}>
          Delete
        </button>
      </header>
      <div ref={bodyRef} className={styles.body}>
        <ConfigSection
          section={props.section}
          value={props.value}
          originalValue={props.originalValue}
          onFieldChange={(field, v) => props.onField(props.instanceKey, field, v)}
        />
        <datalist id={datalistId} data-testid="provider-model-datalist">
          {models.map(m => (
            <option key={m} value={m} />
          ))}
        </datalist>
      </div>
      <footer className={styles.footer}>
        <button
          type="button"
          className={styles.fetchBtn}
          disabled={props.dirty || fetchState.status === 'loading'}
          title={props.dirty ? 'Save first, then fetch models' : undefined}
          onClick={onFetchClick}
        >
          {fetchState.status === 'loading' ? 'Fetching…' : 'Fetch models'}
        </button>
        {fetchState.status === 'ok' && (
          <span className={styles.chipOk}>Connected ✓ ({fetchState.count} models)</span>
        )}
        {fetchState.status === 'err' && (
          <span className={styles.chipErr}>{fetchState.error}</span>
        )}
      </footer>
    </section>
  );
}
```

**Note on `aria-label` selector:** `ConfigSection` renders each field through `TextInput` et al.; those components set the native `<input>`'s label via a `<label>` wrapper. The `aria-label="${modelField.label}"` selector only works if TextInput actually sets `aria-label` on the input — verify by reading `web/src/components/fields/TextInput.tsx` before implementation. If it doesn't, fall back to `bodyRef.current.querySelector<HTMLInputElement>('input[type="text"]:last-of-type')` since the Model field is the last string field in the providers descriptor.

Create `web/src/components/groups/models/ProviderEditor.module.css`:

```css
.editor {
  padding: 16px 24px;
}

.header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border, #30363d);
  margin-bottom: 16px;
}

.breadcrumb {
  flex: 1;
  color: var(--muted, #8b949e);
}

.breadcrumb strong {
  color: var(--text, #c9d1d9);
  font-weight: 600;
}

.deleteBtn {
  padding: 6px 12px;
  background: transparent;
  border: 1px solid var(--error, #f85149);
  border-radius: 3px;
  color: var(--error, #f85149);
  cursor: pointer;
}

.deleteBtn:hover {
  background: var(--error, #f85149);
  color: #fff;
}

.body {
  margin-bottom: 16px;
}

.footer {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--border, #30363d);
}

.fetchBtn {
  padding: 6px 14px;
  background: var(--accent, #FFB800);
  border: 0;
  border-radius: 3px;
  color: var(--accent-fg, #111827);
  cursor: pointer;
  font-weight: 600;
}

.fetchBtn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.chipOk, .chipErr {
  padding: 4px 10px;
  border-radius: 3px;
  font-size: 12px;
}

.chipOk {
  background: rgba(34, 197, 94, 0.15);
  color: var(--success, #3fb950);
}

.chipErr {
  background: rgba(239, 68, 68, 0.15);
  color: var(--error, #f85149);
}
```

Create `web/src/components/groups/models/NewProviderDialog.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react';
import styles from './NewProviderDialog.module.css';

export interface NewProviderDialogProps {
  providerTypes: readonly string[];
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string, providerType: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewProviderDialog({
  providerTypes,
  existingKeys,
  onCancel,
  onCreate,
}: NewProviderDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [key, setKey] = useState('');
  const [providerType, setProviderType] = useState(providerTypes[0] ?? '');
  const [keyError, setKeyError] = useState<string | null>(null);

  useEffect(() => {
    dialogRef.current?.showModal();
  }, []);

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = key.trim();
    if (!trimmed) {
      setKeyError('Instance key is required.');
      return;
    }
    if (!KEY_REGEX.test(trimmed)) {
      setKeyError('Use lowercase letters, digits, underscore. Must start with a letter.');
      return;
    }
    if (existingKeys.has(trimmed)) {
      setKeyError(`An instance named "${trimmed}" already exists.`);
      return;
    }
    if (!providerType) {
      setKeyError('Pick a provider type.');
      return;
    }
    onCreate(trimmed, providerType);
  }

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      onCancel={e => {
        e.preventDefault();
        onCancel();
      }}
      onClose={() => onCancel()}
    >
      <form onSubmit={onSubmit}>
        <header className={styles.header}>
          <h2 className={styles.title}>New provider</h2>
          <span className={styles.spacer} />
          <button
            type="button"
            className={styles.close}
            onClick={onCancel}
            aria-label="Close"
          >
            ✕
          </button>
        </header>
        <div className={styles.body}>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-provider-type">
              Provider type
            </label>
            <select
              id="new-provider-type"
              className={styles.select}
              value={providerType}
              onChange={e => setProviderType(e.currentTarget.value)}
            >
              {providerTypes.map(t => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-provider-key">
              Instance key
            </label>
            <input
              id="new-provider-key"
              className={styles.input}
              value={key}
              placeholder="anthropic_main"
              autoFocus
              onChange={e => {
                setKey(e.currentTarget.value);
                setKeyError(null);
              }}
            />
            <span className={styles.hint}>
              Identifier under <code>providers.*</code>. Lowercase letters, digits, underscore.
            </span>
            {keyError && <span className={styles.error}>{keyError}</span>}
          </div>
        </div>
        <footer className={styles.footer}>
          <button
            type="button"
            className={`${styles.btn} ${styles.secondary}`}
            onClick={onCancel}
          >
            Cancel
          </button>
          <span className={styles.footerSpacer} />
          <button
            type="submit"
            className={`${styles.btn} ${styles.primary}`}
          >
            Create
          </button>
        </footer>
      </form>
    </dialog>
  );
}
```

Create `web/src/components/groups/models/NewProviderDialog.module.css`:

```css
.dialog {
  border: 1px solid var(--border, #30363d);
  border-radius: 6px;
  background: var(--surface, #14171c);
  color: var(--text, #c9d1d9);
  padding: 0;
  min-width: 420px;
}

.dialog::backdrop {
  background: rgba(0, 0, 0, 0.5);
}

.header {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border, #30363d);
}

.title { margin: 0; font-size: 16px; font-weight: 600; }
.spacer { flex: 1; }

.close {
  background: transparent;
  border: 0;
  color: var(--muted, #8b949e);
  cursor: pointer;
  font-size: 16px;
}

.body { padding: 16px; }

.field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; }

.label { font-size: 12px; color: var(--muted, #8b949e); }

.select, .input {
  padding: 6px 8px;
  background: var(--bg, #0b0d11);
  border: 1px solid var(--border, #30363d);
  border-radius: 3px;
  color: var(--text, #c9d1d9);
  font-size: 13px;
}

.hint { font-size: 11px; color: var(--muted, #8b949e); }

.error { font-size: 11px; color: var(--error, #f85149); }

.footer {
  display: flex;
  padding: 12px 16px;
  border-top: 1px solid var(--border, #30363d);
}

.footerSpacer { flex: 1; }

.btn {
  padding: 6px 14px;
  border-radius: 3px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
}

.primary {
  background: var(--accent, #FFB800);
  border: 0;
  color: var(--accent-fg, #111827);
}

.secondary {
  background: transparent;
  border: 1px solid var(--border, #30363d);
  color: var(--text, #c9d1d9);
  margin-right: 8px;
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && pnpm vitest run src/components/groups/models/`

Expected: PASS (5 ModelsSidebar tests + 5 NewProviderDialog tests + 6 ProviderEditor tests = 16 new cases).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/groups/models/
git commit -m "feat(web/models): ModelsSidebar + ProviderEditor + NewProviderDialog"
```

---

## Task 10: Integration — ContentPanel, Sidebar, App.tsx, sections.ts

**Files:**
- Modify: `web/src/shell/sections.ts`
- Modify: `web/src/shell/sections.test.ts`
- Modify: `web/src/components/shell/Sidebar.tsx`
- Modify: `web/src/components/shell/Sidebar.test.tsx`
- Modify: `web/src/components/shell/ContentPanel.tsx`
- Modify: `web/src/components/shell/ContentPanel.test.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Write failing tests**

Append to `web/src/shell/sections.test.ts`:

```ts
it('registers Stage 4b section: providers under models', () => {
  const p = findSection('providers');
  expect(p, 'missing providers').toBeDefined();
  expect(p!.groupId).toBe('models');
  expect(p!.plannedStage).toBe('done');
});

it('models group exposes model then providers in declaration order', () => {
  const models = sectionsInGroup('models');
  expect(models.map(s => s.key)).toEqual(['model', 'providers']);
});
```

Remove or rewrite the existing `'models group exposes model'` test — its assertion `expect(models.map(s => s.key)).toEqual(['model']);` now contradicts the new state. Replace it with the new `['model', 'providers']` assertion above (keep only one, delete the old).

Append to `web/src/components/shell/ContentPanel.test.tsx`:

```tsx
describe('ContentPanel — keyed_map section routing', () => {
  const providersSection: ConfigSection = {
    key: 'providers',
    label: 'Providers',
    group_id: 'models',
    shape: 'keyed_map',
    fields: [
      { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
        enum: ['anthropic'] },
      { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    ],
  };

  it('renders EmptyState when no activeSubKey is selected', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: null,
          configSections: [providersSection],
        })}
      />,
    );
    // EmptyState renders the "No instance selected" message (or similar).
    expect(screen.getByText(/select/i)).toBeInTheDocument();
  });

  it('renders ProviderEditor for a selected keyed-map instance', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'anthropic_main',
          config: {
            providers: {
              anthropic_main: { provider: 'anthropic', api_key: '' },
            },
          } as unknown as Config,
          originalConfig: {
            providers: {
              anthropic_main: { provider: 'anthropic', api_key: '' },
            },
          } as unknown as Config,
          configSections: [providersSection],
        })}
      />,
    );
    expect(screen.getByText(/anthropic_main/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
  });
});
```

Append to `web/src/components/shell/Sidebar.test.tsx`:

```tsx
it('renders ModelsSidebar for the models group (not SectionList)', () => {
  const providersSection: ConfigSection = {
    key: 'providers', label: 'Providers', group_id: 'models', shape: 'keyed_map',
    fields: [
      { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
        enum: ['anthropic'] },
    ],
  };
  const modelSection: ConfigSection = {
    key: 'model', label: 'Default model', group_id: 'models', shape: 'scalar',
    fields: [{ name: 'model', label: 'Model', kind: 'string' }],
  };
  render(
    <Sidebar
      {...baseProps({
        expandedGroups: new Set(['models']),
        configSections: [modelSection, providersSection],
        providerInstances: [{ key: 'anthropic_main', type: 'anthropic' }],
      })}
    />,
  );
  // ModelsSidebar's Default model row
  expect(screen.getByText('Default model')).toBeInTheDocument();
  // Providers header
  expect(screen.getByText('Providers')).toBeInTheDocument();
  // Instance row
  expect(screen.getByText('anthropic_main')).toBeInTheDocument();
  // + New provider button
  expect(screen.getByRole('button', { name: /new provider/i })).toBeInTheDocument();
});
```

Note: `baseProps` in `Sidebar.test.tsx` may need `providerInstances` default. The test file's existing helper already spreads overrides — pass `providerInstances: []` in all existing invocations if tsc complains; the default-add happens in Step 3 when we extend Sidebar's prop type.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && pnpm vitest run src/shell/sections.test.ts src/components/shell/ContentPanel.test.tsx src/components/shell/Sidebar.test.tsx`

Expected: FAIL on the new cases (and Sidebar compilation errors because `providerInstances` prop doesn't exist yet).

- [ ] **Step 3: Register providers in sections.ts**

Edit `web/src/shell/sections.ts`. Append after the `model` entry:

```ts
  { key: 'providers', groupId: 'models', plannedStage: 'done' },
```

Final `SECTIONS` array:

```ts
export const SECTIONS: readonly SectionDef[] = [
  // runtime
  { key: 'storage', groupId: 'runtime', plannedStage: 'done' },
  { key: 'agent', groupId: 'runtime', plannedStage: 'done' },
  { key: 'auxiliary', groupId: 'runtime', plannedStage: 'done' },
  { key: 'terminal', groupId: 'runtime', plannedStage: 'done' },
  // observability
  { key: 'logging', groupId: 'observability', plannedStage: 'done' },
  { key: 'metrics', groupId: 'observability', plannedStage: 'done' },
  { key: 'tracing', groupId: 'observability', plannedStage: 'done' },
  // models
  { key: 'model', groupId: 'models', plannedStage: 'done' },
  { key: 'providers', groupId: 'models', plannedStage: 'done' },
] as const;
```

- [ ] **Step 4: Route the `keyed_map` branch in ContentPanel**

Edit `web/src/components/shell/ContentPanel.tsx`. Import ProviderEditor and EmptyState (EmptyState is already imported). Extend the `activeSubKey` branch so `keyed_map` sections route to ProviderEditor:

```tsx
if (props.activeSubKey) {
    const section = props.configSections.find(
      s => s.key === props.activeSubKey && s.group_id === props.activeGroup,
    );
    // Gateway instance case handled elsewhere; the activeSubKey here refers to
    // a non-gateway section's sub-key OR (for keyed_map) a provider instance.
    if (section) {
      if (section.shape === 'scalar') { /* existing 4a branch */ }
      if (section.shape === 'keyed_map') {
        // activeSubKey is the section key; no instance selected yet.
        return <EmptyState onSelectGroup={props.onSelectGroup} />;
      }
      /* existing map branch */
    }
    // Key didn't match a section — try treating it as a provider-instance key.
    const providersSection = props.configSections.find(s => s.key === 'providers');
    if (providersSection && providersSection.shape === 'keyed_map' && props.activeGroup === 'models') {
      const providers = ((props.config as Record<string, unknown>).providers ?? {}) as Record<string, Record<string, unknown>>;
      const origProviders = ((props.originalConfig as Record<string, unknown>).providers ?? {}) as Record<string, Record<string, unknown>>;
      const instance = providers[props.activeSubKey];
      if (instance) {
        const dirty = !shallowEqualInstance(instance, origProviders[props.activeSubKey]);
        return (
          <ProviderEditor
            sectionKey="providers"
            instanceKey={props.activeSubKey}
            section={providersSection}
            value={instance}
            originalValue={origProviders[props.activeSubKey] ?? {}}
            dirty={dirty}
            onField={(instKey, field, v) =>
              props.onConfigKeyedField('providers', instKey, field, v)
            }
            onDelete={() => props.onConfigKeyedDelete('providers', props.activeSubKey!)}
            fetchModels={() => props.onFetchModels(props.activeSubKey!)}
          />
        );
      }
    }
  }
```

Add the new props to `ContentPanelProps`:

```ts
  onConfigKeyedField: (sectionKey: string, instanceKey: string, field: string, value: unknown) => void;
  onConfigKeyedDelete: (sectionKey: string, instanceKey: string) => void;
  onFetchModels: (instanceKey: string) => Promise<{ models: string[] }>;
```

Add a helper inside the file:

```ts
function shallowEqualInstance(
  a: Record<string, unknown> | undefined,
  b: Record<string, unknown> | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  const ak = Object.keys(a);
  const bk = Object.keys(b);
  if (ak.length !== bk.length) return false;
  for (const k of ak) if (a[k] !== b[k]) return false;
  return true;
}
```

Extend the test-helper `makeProps` in ContentPanel.test.tsx with the new three defaults:

```ts
    onConfigKeyedField: () => {},
    onConfigKeyedDelete: () => {},
    onFetchModels: async () => ({ models: [] }),
```

- [ ] **Step 5: Route the models group in Sidebar**

Edit `web/src/components/shell/Sidebar.tsx`. Add import:

```tsx
import ModelsSidebar from '../groups/models/ModelsSidebar';
```

Add to `SidebarProps`:

```ts
  providerInstances: Array<{ key: string; type: string }>;
  dirtyProviderKeys: Set<string>;
  onNewProvider: () => void;
```

Extend the `g.id === 'gateway'` branch into a three-way check:

```tsx
        const body =
          g.id === 'gateway' ? (
            <GatewaySidebar ... />
          ) : g.id === 'models' ? (
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
            />
          ) : (
            <SectionList ... />
          );
```

Update the existing `baseProps` in Sidebar.test.tsx to include `providerInstances: []`, `dirtyProviderKeys: new Set()`, `onNewProvider: () => {}`.

- [ ] **Step 6: Wire App.tsx**

Edit `web/src/App.tsx`. After `const instances = useMemo(...)` block, add:

```tsx
  const providerInstances = useMemo(() => {
    const p = ((state.config as Record<string, unknown>).providers as Record<string, Record<string, unknown>> | undefined) ?? {};
    return Object.keys(p).sort().map(k => ({
      key: k,
      type: (p[k].provider as string) ?? '',
    }));
  }, [state.config]);

  const dirtyProviderKeys = useMemo(() => {
    // Union of keys in current + original; mark dirty via keyedInstanceDirty.
    const cur = ((state.config as Record<string, unknown>).providers as Record<string, unknown> | undefined) ?? {};
    const orig = ((state.originalConfig as Record<string, unknown>).providers as Record<string, unknown> | undefined) ?? {};
    const keys = new Set<string>([...Object.keys(cur), ...Object.keys(orig)]);
    const out = new Set<string>();
    for (const k of keys) {
      if (keyedInstanceDirty(state, 'providers', k)) out.add(k);
    }
    return out;
  }, [state.config, state.originalConfig, state]);

  const [newProviderDialogOpen, setNewProviderDialogOpen] = useState(false);

  const onFetchProviderModels = useCallback(async (instanceKey: string) => {
    const res = await apiFetch(`/api/providers/${encodeURIComponent(instanceKey)}/models`, {
      method: 'POST',
      schema: ProviderModelsResponseSchema,
    });
    return res;
  }, []);
```

Add imports at the top of App.tsx:

```tsx
import { ProviderModelsResponseSchema } from './api/schemas';
import { keyedInstanceDirty } from './shell/keyedInstances';
import NewProviderDialog from './components/groups/models/NewProviderDialog';
```

Wire the new Sidebar props:

```tsx
        providerInstances={providerInstances}
        dirtyProviderKeys={dirtyProviderKeys}
        onNewProvider={() => setNewProviderDialogOpen(true)}
```

Wire the new ContentPanel props:

```tsx
          onConfigKeyedField={(sectionKey, instanceKey, field, value) =>
            dispatch({ type: 'edit/keyed-instance-field', sectionKey, instanceKey, field, value })
          }
          onConfigKeyedDelete={(sectionKey, instanceKey) => {
            dispatch({ type: 'keyed-instance/delete', sectionKey, instanceKey });
            dispatch({ type: 'shell/selectSub', key: null });
          }}
          onFetchModels={onFetchProviderModels}
```

After the existing `NewInstanceDialog` block, add:

```tsx
      {newProviderDialogOpen && (() => {
        const section = state.configSections.find(s => s.key === 'providers');
        const providerField = section?.fields.find(f => f.name === 'provider');
        const types = (providerField?.enum ?? []) as readonly string[];
        const existingKeys = new Set(Object.keys(
          (state.config as Record<string, unknown>).providers as Record<string, unknown> ?? {}
        ));
        return (
          <NewProviderDialog
            providerTypes={types}
            existingKeys={existingKeys}
            onCancel={() => setNewProviderDialogOpen(false)}
            onCreate={(key, providerType) => {
              dispatch({
                type: 'keyed-instance/create',
                sectionKey: 'providers',
                instanceKey: key,
                initial: { provider: providerType, base_url: '', api_key: '', model: '' },
              });
              dispatch({ type: 'shell/selectGroup', group: 'models' });
              dispatch({ type: 'shell/selectSub', key });
              setNewProviderDialogOpen(false);
            }}
          />
        );
      })()}
```

Import NewProviderDialog at the top of App.tsx.

- [ ] **Step 7: Run the full web suite**

Run: `cd web && pnpm test && pnpm type-check && pnpm lint`

Expected: all PASS. Test count jumps from 168 (post-Stage-4a.1) to 204 (Stage 4b adds: 5 schema + 6 state + 5 keyedInstances + 5 ModelsSidebar + 5 NewProviderDialog + 6 ProviderEditor + 2 sections — 1 deleted + 2 ContentPanel + 1 Sidebar = 36 new). Any number outside 202-207 — stop and investigate.

- [ ] **Step 8: Commit**

```bash
git add web/src/shell/sections.ts web/src/shell/sections.test.ts web/src/components/shell/Sidebar.tsx web/src/components/shell/Sidebar.test.tsx web/src/components/shell/ContentPanel.tsx web/src/components/shell/ContentPanel.test.tsx web/src/App.tsx
git commit -m "feat(web): integrate Models group — Sidebar, ContentPanel, App dispatchers"
```

---

## Task 11: Gauntlet + webroot + smoke doc

**Files:**
- Rebuilt: `api/webroot/` (generated by `make web-check`)
- Modify: `docs/smoke/web-config.md`

- [ ] **Step 1: Run the Go test suite**

Run: `go test ./config/descriptor/... ./api/... ./provider/factory/...`

Expected: PASS on every package.

- [ ] **Step 2: Run the web test suite**

Run: `cd web && pnpm test`

Expected: 204 tests pass (Stage 4a wrapped at 168; Stage 4b adds 36 new).

- [ ] **Step 3: Type-check + lint**

Run: `cd web && pnpm type-check && pnpm lint`

Expected: both PASS with zero output.

- [ ] **Step 4: Rebuild the web bundle**

Run: `make web-check`

Expected: Vite build succeeds; `api/webroot/` re-synced from `web/dist/`.

- [ ] **Step 5: Append the Stage 4b smoke-doc section**

Append to `docs/smoke/web-config.md`:

```markdown
## Stage 4b · Providers editor + fetch-models

- Sidebar Models group shows "Default model" at the top and a "Providers" header below with the instance list and "+ New provider" button.
- Click "+ New provider" — modal opens. Type `BAD KEY` + click Create → red error "Use lowercase letters, digits, underscore". Type `anthropic_main`, pick type `anthropic`, click Create → modal closes, sidebar shows the row, URL becomes `#models/anthropic_main`, editor shows 4 fields (provider type enum pre-selected as `anthropic`).
- Fill Base URL, API key (real key), Model. Click Save — toast "Saved". Click Fetch models — green chip "Connected ✓ (N models)". Typing in the Model field shows autocomplete suggestions from the returned list.
- Flip API key to garbage, click Save, click Fetch models — red chip with the upstream error ("401 unauthorized" or similar).
- Re-visit `#models/anthropic_main`. API key field is blanked (GET redacts). Click Save without typing into API key — stored key is preserved (same contract as `storage.postgres_url` and `auxiliary.api_key`).
- Click Delete, confirm → sidebar row disappears, pane returns to EmptyState. Save — YAML `providers:` block updates (entry removed).
- `POST /api/providers/<name>/models` returns `{"models": ["id", ...]}` on success. Errors: 404 unknown name, 400 factory rejection, 501 non-ModelLister provider, 502 upstream.
```

- [ ] **Step 6: Commit**

Stage ONLY `api/webroot/` and `docs/smoke/web-config.md` (NOT unrelated working-tree WIP):

```bash
git add api/webroot/ docs/smoke/web-config.md
git commit -m "chore(web): rebuild webroot + Stage 4b smoke flow"
```

- [ ] **Step 7: Manual smoke (required before sign-off)**

Rebuild the binary so the new webroot is embedded:

```bash
go build -o bin/hermind ./cmd/hermind   # or your usual build command
./bin/hermind web --addr=127.0.0.1:9119 --no-browser
```

Hard-refresh the browser and work through the bullets from Step 5. Every bullet must pass before calling Stage 4b done.

---

## Completion checklist

Before calling Stage 4b complete, verify:

- [ ] `go test ./config/descriptor/... ./api/... ./provider/factory/...` — PASS
- [ ] `cd web && pnpm test` — PASS (~204 tests)
- [ ] `cd web && pnpm type-check` — PASS
- [ ] `cd web && pnpm lint` — zero warnings
- [ ] `make web-check` — PASS
- [ ] Manual smoke from Task 11 Step 7 — all bullets pass
- [ ] `docs/smoke/web-config.md` has the new Stage 4b section
- [ ] `git status --short` — no dangling Stage-4b files (unrelated in-progress stages may remain)

Once all boxes are checked, Stage 4b is complete. Stage 4c (fallback providers list editor) picks up from here; it introduces `ShapeList` and a list-editor component, reusing the ConfigSection field renderers.
