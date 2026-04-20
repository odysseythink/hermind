# Web Config Schema Infrastructure (Stage 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the schema infrastructure (Go descriptor package + `GET /api/config/schema` + generic React renderer) that stages 3–7 will consume, with the Storage section as the end-to-end vertical slice.

**Architecture:** New `config/descriptor` Go package mirrors `gateway/platforms` (types + registry + per-section `init()` register calls). REST returns sections sorted by key. Frontend adds one new field kind (`float`), one optional `SecretInput` prop (`disableReveal`), one generic `ConfigSection` renderer, a `shell/sections.ts` registry, and a single new reducer action. Routing `#runtime/storage` renders the Storage editor; all other routes keep Stage-1 behavior.

**Tech Stack:** Go 1.x + `gopkg.in/yaml.v3` + `go-chi/chi`. TypeScript 5, React 18, Vite 5, Vitest 2, zod, `@testing-library/react`, pnpm 10. No new runtime dependencies.

**Spec:** `docs/superpowers/specs/2026-04-20-web-schema-infra-design.md`.

**Branch:** Work on `main` directly (this repo is trunk-based; see prior stages).

**Always run from repo root** unless a task says otherwise.

**Running tests:**
- Go: `go test ./config/descriptor/... ./api/...`
- Web: `cd web && pnpm test` (one-shot) or `cd web && pnpm test:watch`

---

## Task 1: `config/descriptor` package — types and registry

**Files:**
- Create: `config/descriptor/descriptor.go`

**Why:** Foundation for every downstream task. Mirrors `gateway/platforms.FieldSpec` / `Descriptor` / `Register` / `All` so existing mental models carry over, but carries `VisibleWhen` and a new `FieldFloat` kind.

- [ ] **Step 1: Create the package file**

Write `config/descriptor/descriptor.go`:

```go
// Package descriptor hosts the schema descriptors used by /api/config/schema
// and the generic React section editor in web/src/components/ConfigSection.tsx.
//
// Each non-platform config section ships a storage.go / agent.go / …
// file whose init() calls Register. The REST handler exposes All()
// so the frontend can render every section without hand-coding its fields.
package descriptor

import "sort"

// FieldKind enumerates the value shapes a descriptor field can carry.
type FieldKind int

const (
	// FieldUnknown is the zero value; descriptor authors must set Kind
	// explicitly. The invariants test rejects any field left at FieldUnknown.
	FieldUnknown FieldKind = iota
	FieldString
	FieldInt
	FieldBool
	FieldSecret
	FieldEnum
	FieldFloat
)

// String returns a lowercase name suitable for JSON ("string", "secret", …).
func (k FieldKind) String() string {
	switch k {
	case FieldString:
		return "string"
	case FieldInt:
		return "int"
	case FieldBool:
		return "bool"
	case FieldSecret:
		return "secret"
	case FieldEnum:
		return "enum"
	case FieldFloat:
		return "float"
	}
	return "unknown"
}

// Predicate expresses "show this field only when <Field> equals <Equals>".
// Stage 2 supports exactly one equality check; boolean algebra is YAGNI.
type Predicate struct {
	Field  string
	Equals any
}

// FieldSpec describes one configurable field of a Section.
type FieldSpec struct {
	Name        string     // yaml key: "sqlite_path"
	Label       string     // human-readable label
	Help        string     // optional one-line hint
	Kind        FieldKind
	Required    bool
	Default     any        // nil when none
	Enum        []string   // only for FieldEnum
	VisibleWhen *Predicate // nil = always visible
}

// Section is the schema for one top-level config.Config field
// (e.g. config.Storage at yaml key "storage").
type Section struct {
	Key     string      // "storage" — matches the yaml tag on config.Config
	Label   string
	Summary string
	GroupID string      // "runtime" — which shell group hosts this section
	Fields  []FieldSpec
}

var registry = map[string]Section{}

// Register installs s under s.Key, overwriting any prior entry.
// Callers invoke this from init() in a per-section file.
func Register(s Section) {
	registry[s.Key] = s
}

// Get returns the section registered at key. The second return value
// is false when key is unknown.
func Get(key string) (Section, bool) {
	s, ok := registry[key]
	return s, ok
}

// All returns every registered section sorted by Key so the JSON
// response is deterministic.
func All() []Section {
	out := make([]Section, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build ./config/descriptor/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add config/descriptor/descriptor.go
git commit -m "feat(config/descriptor): package skeleton with FieldKind, Predicate, Section, registry"
```

---

## Task 2: Registry + invariants test

**Files:**
- Create: `config/descriptor/descriptor_test.go`

**Why:** Lock the behavior of `Register` / `Get` / `All` and the invariants every section must satisfy before stage-3+ sections start landing.

- [ ] **Step 1: Write the failing tests**

Write `config/descriptor/descriptor_test.go`:

```go
package descriptor

import (
	"sort"
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	key := "__test_register"
	defer delete(registry, key)

	want := Section{Key: key, Label: "t", GroupID: "runtime", Fields: []FieldSpec{{Name: "x", Label: "X", Kind: FieldString}}}
	Register(want)

	got, ok := Get(key)
	if !ok {
		t.Fatalf("Get(%q) returned ok=false", key)
	}
	if got.Label != want.Label {
		t.Errorf("Label = %q, want %q", got.Label, want.Label)
	}
}

func TestAllReturnsSortedByKey(t *testing.T) {
	keys := []string{"__t_bbb", "__t_aaa", "__t_ccc"}
	for _, k := range keys {
		Register(Section{Key: k, Label: k, GroupID: "runtime", Fields: []FieldSpec{{Name: "x", Label: "X", Kind: FieldString}}})
	}
	defer func() {
		for _, k := range keys {
			delete(registry, k)
		}
	}()

	all := All()
	var got []string
	for _, s := range all {
		if len(s.Key) > 4 && s.Key[:4] == "__t_" {
			got = append(got, s.Key)
		}
	}
	want := append([]string(nil), keys...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("got %d matching keys, want %d: %v vs %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("All() order = %v, want %v", got, want)
		}
	}
}

func TestFieldKindString(t *testing.T) {
	cases := []struct {
		k    FieldKind
		want string
	}{
		{FieldString, "string"},
		{FieldInt, "int"},
		{FieldBool, "bool"},
		{FieldSecret, "secret"},
		{FieldEnum, "enum"},
		{FieldFloat, "float"},
		{FieldUnknown, "unknown"},
		{FieldKind(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.k.String(); got != tc.want {
			t.Errorf("(%d).String() = %q, want %q", tc.k, got, tc.want)
		}
	}
}

// TestSectionInvariants walks every registered section and enforces the
// guarantees the frontend and redaction loop depend on. Stage 2 has only
// one registered section (Storage); this test protects every stage-3+
// addition from landing broken.
func TestSectionInvariants(t *testing.T) {
	for _, s := range All() {
		if s.Key == "" {
			t.Errorf("section with empty Key: %+v", s)
		}
		if s.Label == "" {
			t.Errorf("section %q: empty Label", s.Key)
		}
		if s.GroupID == "" {
			t.Errorf("section %q: empty GroupID", s.Key)
		}
		if len(s.Fields) == 0 {
			t.Errorf("section %q: no Fields", s.Key)
		}
		names := map[string]bool{}
		for _, f := range s.Fields {
			if f.Kind == FieldUnknown {
				t.Errorf("section %q field %q: Kind is FieldUnknown", s.Key, f.Name)
			}
			if f.Name == "" {
				t.Errorf("section %q: field with empty Name", s.Key)
			}
			if f.Label == "" {
				t.Errorf("section %q field %q: empty Label", s.Key, f.Name)
			}
			if names[f.Name] {
				t.Errorf("section %q: duplicate field Name %q", s.Key, f.Name)
			}
			names[f.Name] = true
			if f.Kind == FieldEnum && len(f.Enum) == 0 {
				t.Errorf("section %q field %q: FieldEnum with empty Enum", s.Key, f.Name)
			}
		}
		// VisibleWhen.Field must reference a sibling field declared in the
		// same section. Evaluated after the names map is fully built so
		// forward references are legal.
		for _, f := range s.Fields {
			if f.VisibleWhen == nil {
				continue
			}
			if !names[f.VisibleWhen.Field] {
				t.Errorf("section %q field %q: VisibleWhen.Field %q is not a sibling field",
					s.Key, f.Name, f.VisibleWhen.Field)
			}
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm they pass against the empty registry**

```bash
go test ./config/descriptor/...
```

Expected: PASS. `TestSectionInvariants` iterates `All()` which is empty at this point — the loop body never runs, so the test passes. When Task 3 lands the Storage section, the invariants fire for real.

- [ ] **Step 3: Commit**

```bash
git add config/descriptor/descriptor_test.go
git commit -m "test(config/descriptor): registry + invariants test harness"
```

---

## Task 3: Storage section descriptor + test

**Files:**
- Create: `config/descriptor/storage.go`
- Create: `config/descriptor/storage_test.go`

**Why:** The vertical slice. Exercises `VisibleWhen` with a 3-field flat struct that matches `config.StorageConfig`.

- [ ] **Step 1: Write the Storage-specific test first**

Write `config/descriptor/storage_test.go`:

```go
package descriptor

import "testing"

func TestStorageSectionRegistered(t *testing.T) {
	s, ok := Get("storage")
	if !ok {
		t.Fatalf("Get(\"storage\") returned ok=false — did storage.go init() register?")
	}
	if s.GroupID != "runtime" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "runtime")
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}

	wantFields := map[string]FieldKind{
		"driver":       FieldEnum,
		"sqlite_path":  FieldString,
		"postgres_url": FieldSecret,
	}
	gotFields := map[string]FieldKind{}
	for _, f := range s.Fields {
		gotFields[f.Name] = f.Kind
	}
	for name, kind := range wantFields {
		got, ok := gotFields[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if got != kind {
			t.Errorf("field %q: Kind = %s, want %s", name, got, kind)
		}
	}
}

func TestStorageDriverIsEnumWithSQLiteAndPostgres(t *testing.T) {
	s, _ := Get("storage")
	var driver *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "driver" {
			driver = &s.Fields[i]
			break
		}
	}
	if driver == nil {
		t.Fatal("driver field not found")
	}
	if driver.Kind != FieldEnum {
		t.Fatalf("driver.Kind = %s, want enum", driver.Kind)
	}
	want := map[string]bool{"sqlite": true, "postgres": true}
	for _, v := range driver.Enum {
		delete(want, v)
	}
	if len(want) > 0 {
		t.Errorf("driver.Enum missing %v, got %v", want, driver.Enum)
	}
	if driver.Default != "sqlite" {
		t.Errorf("driver.Default = %v, want \"sqlite\"", driver.Default)
	}
	if !driver.Required {
		t.Error("driver.Required = false, want true")
	}
}

func TestStoragePathFieldsAreGatedOnDriver(t *testing.T) {
	s, _ := Get("storage")
	cases := map[string]string{
		"sqlite_path":  "sqlite",
		"postgres_url": "postgres",
	}
	for fieldName, wantDriver := range cases {
		var f *FieldSpec
		for i := range s.Fields {
			if s.Fields[i].Name == fieldName {
				f = &s.Fields[i]
				break
			}
		}
		if f == nil {
			t.Errorf("field %q not found", fieldName)
			continue
		}
		if f.VisibleWhen == nil {
			t.Errorf("field %q: VisibleWhen is nil", fieldName)
			continue
		}
		if f.VisibleWhen.Field != "driver" {
			t.Errorf("field %q: VisibleWhen.Field = %q, want \"driver\"", fieldName, f.VisibleWhen.Field)
		}
		if f.VisibleWhen.Equals != wantDriver {
			t.Errorf("field %q: VisibleWhen.Equals = %v, want %q", fieldName, f.VisibleWhen.Equals, wantDriver)
		}
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./config/descriptor/... -run TestStorage
```

Expected: FAIL with `Get("storage") returned ok=false`.

- [ ] **Step 3: Implement storage.go**

Write `config/descriptor/storage.go`:

```go
package descriptor

func init() {
	Register(Section{
		Key:     "storage",
		Label:   "Storage",
		Summary: "Where hermind keeps conversation history and agent state.",
		GroupID: "runtime",
		Fields: []FieldSpec{
			{
				Name:     "driver",
				Label:    "Driver",
				Help:     "Storage backend to use.",
				Kind:     FieldEnum,
				Required: true,
				Default:  "sqlite",
				Enum:     []string{"sqlite", "postgres"},
			},
			{
				Name:        "sqlite_path",
				Label:       "SQLite path",
				Help:        "Filesystem path to the SQLite database file.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "driver", Equals: "sqlite"},
			},
			{
				Name:        "postgres_url",
				Label:       "Postgres URL",
				Help:        "postgres://user:pass@host/db connection string.",
				Kind:        FieldSecret,
				VisibleWhen: &Predicate{Field: "driver", Equals: "postgres"},
			},
		},
	})
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test ./config/descriptor/...
```

Expected: PASS (all four tests: `TestRegisterAndGet`, `TestAllReturnsSortedByKey`, `TestFieldKindString`, `TestStorageSectionRegistered`, `TestStorageDriverIsEnumWithSQLiteAndPostgres`, `TestStoragePathFieldsAreGatedOnDriver`, `TestSectionInvariants`). `TestSectionInvariants` now iterates a real section.

- [ ] **Step 5: Commit**

```bash
git add config/descriptor/storage.go config/descriptor/storage_test.go
git commit -m "feat(config/descriptor): Storage section with visible_when path fields"
```

---

## Task 4: ConfigSchema DTOs

**Files:**
- Modify: `api/dto.go`

**Why:** Server-side JSON shape for `/api/config/schema`. Kept separate from `SchemaFieldDTO`/`SchemaDescriptorDTO` (platform schema) to avoid conflating two responsibilities; the types look similar but diverge when stage 5 adds nested sections.

- [ ] **Step 1: Append new DTOs to `api/dto.go`**

Add the following after the existing `PlatformsSchemaResponse` block (the file currently ends at line 172 around `ApplyResult`; append at the end):

```go
// PredicateDTO is the JSON shape of descriptor.Predicate.
type PredicateDTO struct {
	Field  string `json:"field"`
	Equals any    `json:"equals"`
}

// ConfigFieldDTO is one field of a ConfigSectionDTO.
type ConfigFieldDTO struct {
	Name        string        `json:"name"`
	Label       string        `json:"label"`
	Help        string        `json:"help,omitempty"`
	Kind        string        `json:"kind"`
	Required    bool          `json:"required,omitempty"`
	Default     any           `json:"default,omitempty"`
	Enum        []string      `json:"enum,omitempty"`
	VisibleWhen *PredicateDTO `json:"visible_when,omitempty"`
}

// ConfigSectionDTO is one section in the config schema response.
type ConfigSectionDTO struct {
	Key     string           `json:"key"`
	Label   string           `json:"label"`
	Summary string           `json:"summary,omitempty"`
	GroupID string           `json:"group_id"`
	Fields  []ConfigFieldDTO `json:"fields"`
}

// ConfigSchemaResponse is the payload for GET /api/config/schema.
type ConfigSchemaResponse struct {
	Sections []ConfigSectionDTO `json:"sections"`
}
```

- [ ] **Step 2: Verify it builds**

```bash
go build ./api/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add api/dto.go
git commit -m "feat(api/dto): DTOs for /api/config/schema"
```

---

## Task 5: `GET /api/config/schema` handler + test

**Files:**
- Create: `api/handlers_config_schema.go`
- Create: `api/handlers_config_schema_test.go`
- Modify: `api/server.go:147` (add route)

**Why:** Serves the descriptors the frontend reads at boot.

- [ ] **Step 1: Write the failing test**

Write `api/handlers_config_schema_test.go`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	_ "github.com/odysseythink/hermind/config/descriptor"
)

func TestConfigSchema_IncludesStorageSection(t *testing.T) {
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
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body api.ConfigSchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}

	var storage *api.ConfigSectionDTO
	for i := range body.Sections {
		if body.Sections[i].Key == "storage" {
			storage = &body.Sections[i]
			break
		}
	}
	if storage == nil {
		t.Fatalf("missing storage section; got sections = %+v", body.Sections)
	}
	if storage.GroupID != "runtime" {
		t.Errorf("storage.group_id = %q, want \"runtime\"", storage.GroupID)
	}

	names := map[string]api.ConfigFieldDTO{}
	for _, f := range storage.Fields {
		names[f.Name] = f
	}
	if _, ok := names["driver"]; !ok {
		t.Error("missing driver field")
	}
	path := names["sqlite_path"]
	if path.VisibleWhen == nil {
		t.Fatal("sqlite_path.visible_when is nil")
	}
	if path.VisibleWhen.Field != "driver" || path.VisibleWhen.Equals != "sqlite" {
		t.Errorf("sqlite_path.visible_when = %+v, want {field: driver, equals: sqlite}", path.VisibleWhen)
	}
	if names["postgres_url"].Kind != "secret" {
		t.Errorf("postgres_url.kind = %q, want \"secret\"", names["postgres_url"].Kind)
	}
}

func TestConfigSchema_SectionsSortedByKey(t *testing.T) {
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	req := httptest.NewRequest("GET", "/api/config/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var body api.ConfigSchemaResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)

	keys := make([]string, len(body.Sections))
	for i, s := range body.Sections {
		keys[i] = s.Key
	}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	for i := range keys {
		if keys[i] != sorted[i] {
			t.Fatalf("sections not sorted by key: %v", keys)
		}
	}
}
```

- [ ] **Step 2: Run the test to confirm the route is missing**

```bash
go test ./api/... -run TestConfigSchema
```

Expected: FAIL with 404 (route not registered yet).

- [ ] **Step 3: Implement the handler**

Write `api/handlers_config_schema.go`:

```go
package api

import (
	"net/http"

	"github.com/odysseythink/hermind/config/descriptor"
)

// handleConfigSchema responds to GET /api/config/schema with every
// section registered via descriptor.Register. The response is sorted by
// section key so the frontend can render a deterministic navigation.
func (s *Server) handleConfigSchema(w http.ResponseWriter, _ *http.Request) {
	all := descriptor.All()
	out := ConfigSchemaResponse{Sections: make([]ConfigSectionDTO, 0, len(all))}
	for _, sec := range all {
		fields := make([]ConfigFieldDTO, 0, len(sec.Fields))
		for _, f := range sec.Fields {
			dto := ConfigFieldDTO{
				Name:     f.Name,
				Label:    f.Label,
				Help:     f.Help,
				Kind:     f.Kind.String(),
				Required: f.Required,
				Default:  f.Default,
				Enum:     f.Enum,
			}
			if f.VisibleWhen != nil {
				dto.VisibleWhen = &PredicateDTO{
					Field:  f.VisibleWhen.Field,
					Equals: f.VisibleWhen.Equals,
				}
			}
			fields = append(fields, dto)
		}
		out.Sections = append(out.Sections, ConfigSectionDTO{
			Key:     sec.Key,
			Label:   sec.Label,
			Summary: sec.Summary,
			GroupID: sec.GroupID,
			Fields:  fields,
		})
	}
	writeJSON(w, out)
}
```

- [ ] **Step 4: Register the route in `api/server.go`**

Open `api/server.go`. Inside `buildRouter()`, locate the line (around line 147):

```go
r.Get("/platforms/schema", s.handlePlatformsSchema)
```

Add a line directly above it:

```go
r.Get("/config/schema", s.handleConfigSchema)
r.Get("/platforms/schema", s.handlePlatformsSchema)
```

- [ ] **Step 5: Run the test to confirm it passes**

```bash
go test ./api/... -run TestConfigSchema
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/handlers_config_schema.go api/handlers_config_schema_test.go api/server.go
git commit -m "feat(api): GET /api/config/schema serves registered section descriptors"
```

---

## Task 6: Extend `redactSecrets` for registered sections

**Files:**
- Modify: `api/handlers_config.go:36-64` (`redactSecrets`)
- Modify: `api/handlers_config_test.go` (append one test)

**Why:** Stage 2 introduces `storage.postgres_url` as a `FieldSecret`. `GET /api/config` must blank it like it already blanks platform secrets.

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_config_test.go`:

```go
func TestHandleConfigGet_RedactsSectionSecretFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Driver = "postgres"
	cfg.Storage.PostgresURL = "postgres://user:pass@host/db"

	srv, err := NewServer(&ServerOpts{
		Config: cfg,
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/config", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	storage, ok := body.Config["storage"].(map[string]any)
	if !ok {
		t.Fatalf("storage section missing: %+v", body.Config)
	}
	if got := storage["postgres_url"]; got != "" {
		t.Errorf("postgres_url = %v, want blank (redacted)", got)
	}
	// Sanity check: non-secret fields are NOT blanked.
	if got := storage["driver"]; got != "postgres" {
		t.Errorf("driver = %v, want \"postgres\"", got)
	}
}
```

Leave the existing `_ "github.com/odysseythink/hermind/config/descriptor"` blank-import check to the compiler — Task 5's test file already imports it, so the registry is loaded at test time.

- [ ] **Step 2: Run it to confirm it fails**

```bash
go test ./api/... -run TestHandleConfigGet_RedactsSectionSecretFields
```

Expected: FAIL — `postgres_url` still holds the plaintext because `redactSecrets` doesn't know about sections yet.

- [ ] **Step 3: Extend `redactSecrets` in `api/handlers_config.go`**

Open `api/handlers_config.go`. At the top, add the descriptor import:

```go
import (
	"encoding/json"
	"io"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/config/descriptor"
	"github.com/odysseythink/hermind/gateway/platforms"
)
```

Replace the `redactSecrets` function (currently ending at line 64) with:

```go
// redactSecrets blanks every secret field in m, covering two universes:
//
//  1. gateway.platforms[*].options — fields of Kind FieldSecret on the
//     platform descriptor registered in gateway/platforms.
//  2. m[section.Key][field.Name] — fields of Kind FieldSecret on every
//     config section registered in config/descriptor.
//
// Silently ignores unknown types, missing keys, or non-map values —
// we're redacting defensively, not validating.
func redactSecrets(m map[string]any) {
	redactPlatformSecrets(m)
	redactSectionSecrets(m)
}

func redactPlatformSecrets(m map[string]any) {
	gw, _ := m["gateway"].(map[string]any)
	plats, _ := gw["platforms"].(map[string]any)
	for _, raw := range plats {
		inst, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := inst["type"].(string)
		if typ == "" {
			continue
		}
		d, ok := platforms.Get(typ)
		if !ok {
			continue
		}
		opts, _ := inst["options"].(map[string]any)
		if opts == nil {
			continue
		}
		for _, f := range d.Fields {
			if f.Kind == platforms.FieldSecret {
				if _, present := opts[f.Name]; present {
					opts[f.Name] = ""
				}
			}
		}
	}
}

func redactSectionSecrets(m map[string]any) {
	for _, sec := range descriptor.All() {
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

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test ./api/... -run TestHandleConfigGet
```

Expected: both the existing platform-secret test and the new section-secret test PASS.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go
git commit -m "feat(api): redactSecrets covers registered config sections"
```

---

## Task 7: Extend `preserveSecrets` for registered sections

**Files:**
- Modify: `api/handlers_config.go:104-127` (`preserveSecrets`)
- Modify: `api/handlers_config_test.go` (append one test)

**Why:** When the frontend sends back `storage.postgres_url = ""` (the redacted value), the server must copy the previous secret back into the config, mirroring platform-secret behavior.

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_config_test.go`:

```go
func TestHandleConfigPut_PreservesSectionSecretOnBlank(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Driver = "postgres"
	cfg.Storage.PostgresURL = "postgres://user:pass@host/db"

	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")

	srv, err := NewServer(&ServerOpts{
		Config:     cfg,
		ConfigPath: path,
		Token:      "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// The frontend ships back the redacted blank for postgres_url; the
	// rest of the config round-trips unchanged.
	putBody := strings.NewReader(`{"config":{"storage":{"driver":"postgres","postgres_url":""}}}`)
	req := httptest.NewRequest("PUT", "/api/config", putBody)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", w.Code, w.Body.String())
	}

	if got := cfg.Storage.PostgresURL; got != "postgres://user:pass@host/db" {
		t.Errorf("PostgresURL = %q, want preserved secret", got)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

```bash
go test ./api/... -run TestHandleConfigPut_PreservesSectionSecretOnBlank
```

Expected: FAIL — `PostgresURL` is empty because `preserveSecrets` doesn't cover sections.

- [ ] **Step 3: Extend `preserveSecrets`**

Open `api/handlers_config.go`. Replace the `preserveSecrets` function with:

```go
// preserveSecrets copies every blank secret in updated back from current,
// covering both platform secrets (gateway.platforms[*].options) and
// section secrets registered in config/descriptor. Keys missing from
// current (new platforms, new providers, …) are left as-is.
func preserveSecrets(updated, current *config.Config) {
	preservePlatformSecrets(updated, current)
	preserveSectionSecrets(updated, current)
}

func preservePlatformSecrets(updated, current *config.Config) {
	for key, newPC := range updated.Gateway.Platforms {
		curPC, ok := current.Gateway.Platforms[key]
		if !ok {
			continue
		}
		d, ok := platforms.Get(newPC.Type)
		if !ok {
			continue
		}
		if newPC.Options == nil {
			newPC.Options = map[string]string{}
		}
		for _, f := range d.Fields {
			if f.Kind != platforms.FieldSecret {
				continue
			}
			if newPC.Options[f.Name] == "" {
				newPC.Options[f.Name] = curPC.Options[f.Name]
			}
		}
		updated.Gateway.Platforms[key] = newPC
	}
}

// preserveSectionSecrets round-trips blanks for every FieldSecret on a
// registered section. The mapping from (section key, field name) to the
// Go struct field is done via a YAML round-trip: marshal both configs
// into map[string]any, mutate updated's map, re-unmarshal back into
// updated. This avoids reflection-over-struct-tags gymnastics at the
// cost of two marshal/unmarshal cycles — acceptable because PUT is cold.
func preserveSectionSecrets(updated, current *config.Config) {
	sections := descriptor.All()
	if len(sections) == 0 {
		return
	}
	// Detect any secret field that's blank in updated — cheap reflection
	// would also work, but YAML round-trip keeps this keyed on yaml tags,
	// same as the rest of the config handler.
	updBytes, err := yaml.Marshal(updated)
	if err != nil {
		return
	}
	curBytes, err := yaml.Marshal(current)
	if err != nil {
		return
	}
	var updM, curM map[string]any
	if err := yaml.Unmarshal(updBytes, &updM); err != nil {
		return
	}
	if err := yaml.Unmarshal(curBytes, &curM); err != nil {
		return
	}

	changed := false
	for _, sec := range sections {
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
	if !changed {
		return
	}

	reBytes, err := yaml.Marshal(updM)
	if err != nil {
		return
	}
	_ = yaml.Unmarshal(reBytes, updated)
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test ./api/...
```

Expected: the new test passes, existing tests still pass.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go
git commit -m "feat(api): preserveSecrets round-trips registered section secrets"
```

---

## Task 8: Zod schemas for the config-schema response

**Files:**
- Modify: `web/src/api/schemas.ts` (append)
- Modify: `web/src/api/schemas.test.ts` (append happy + sad tests)

**Why:** The frontend validates the `/api/config/schema` response shape at boot. Zod schemas also drive the TypeScript types.

- [ ] **Step 1: Write the failing tests**

Append to `web/src/api/schemas.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { ConfigSchemaResponseSchema } from './schemas';

describe('ConfigSchemaResponseSchema', () => {
  it('accepts a storage section with visible_when', () => {
    const good = {
      sections: [
        {
          key: 'storage',
          label: 'Storage',
          summary: 'Where hermind keeps data.',
          group_id: 'runtime',
          fields: [
            { name: 'driver', label: 'Driver', kind: 'enum',
              required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
            { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
              visible_when: { field: 'driver', equals: 'sqlite' } },
            { name: 'postgres_url', label: 'Postgres URL', kind: 'secret',
              visible_when: { field: 'driver', equals: 'postgres' } },
          ],
        },
      ],
    };
    expect(() => ConfigSchemaResponseSchema.parse(good)).not.toThrow();
  });

  it('rejects a response missing sections', () => {
    const bad = { whatever: [] };
    expect(() => ConfigSchemaResponseSchema.parse(bad)).toThrow();
  });

  it('rejects a field with unknown kind', () => {
    const bad = {
      sections: [
        { key: 's', label: 'S', group_id: 'runtime',
          fields: [{ name: 'x', label: 'X', kind: 'mystery' }] },
      ],
    };
    expect(() => ConfigSchemaResponseSchema.parse(bad)).toThrow();
  });
});
```

- [ ] **Step 2: Run it to confirm it fails**

```bash
cd web && pnpm test -- schemas
```

Expected: FAIL (the schema doesn't exist yet).

- [ ] **Step 3: Add the zod schemas**

Append to `web/src/api/schemas.ts`:

```ts
// Config section kinds produced by descriptor.FieldKind.String(). Adds
// 'float' on top of the platform FieldKind set.
export const ConfigFieldKindSchema = z.enum([
  'string', 'int', 'bool', 'secret', 'enum', 'float',
]);
export type ConfigFieldKind = z.infer<typeof ConfigFieldKindSchema>;

export const ConfigPredicateSchema = z.object({
  field: z.string(),
  equals: z.unknown(),
});
export type ConfigPredicate = z.infer<typeof ConfigPredicateSchema>;

export const ConfigFieldSchema = z.object({
  name: z.string(),
  label: z.string(),
  help: z.string().optional(),
  kind: ConfigFieldKindSchema,
  required: z.boolean().optional(),
  default: z.unknown().optional(),
  enum: z.array(z.string()).optional(),
  visible_when: ConfigPredicateSchema.optional(),
});
export type ConfigField = z.infer<typeof ConfigFieldSchema>;

export const ConfigSectionSchema = z.object({
  key: z.string(),
  label: z.string(),
  summary: z.string().optional(),
  group_id: z.string(),
  fields: z.array(ConfigFieldSchema),
});
export type ConfigSection = z.infer<typeof ConfigSectionSchema>;

export const ConfigSchemaResponseSchema = z.object({
  sections: z.array(ConfigSectionSchema),
});
export type ConfigSchemaResponse = z.infer<typeof ConfigSchemaResponseSchema>;
```

- [ ] **Step 4: Run the tests to confirm they pass**

```bash
cd web && pnpm test -- schemas
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/api/schemas.ts web/src/api/schemas.test.ts
git commit -m "feat(web/api): zod schemas for /api/config/schema response"
```

---

## Task 9: `edit/config-field` reducer action + state types

**Files:**
- Modify: `web/src/state.ts`
- Modify: `web/src/state.test.ts`

**Why:** Single immutable action that stages 3–7 will also use. Depth-2 merge `config[sectionKey][field] = value`.

- [ ] **Step 1: Write the failing test**

Append to `web/src/state.test.ts`:

```ts
describe('edit/config-field', () => {
  const base: AppState = {
    ...initialState,
    status: 'ready',
    config: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x.sqlite' } },
    originalConfig: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x.sqlite' } },
  };

  it('sets a field on an existing section', () => {
    const next = reducer(base, {
      type: 'edit/config-field', sectionKey: 'storage', field: 'driver', value: 'postgres',
    });
    expect(next.config.storage).toEqual({
      driver: 'postgres',
      sqlite_path: '/var/db/x.sqlite',
    });
    // Immutability: original state is untouched.
    expect(base.config.storage).toEqual({
      driver: 'sqlite',
      sqlite_path: '/var/db/x.sqlite',
    });
  });

  it('creates the section object when missing', () => {
    const empty = { ...base, config: {} };
    const next = reducer(empty, {
      type: 'edit/config-field', sectionKey: 'storage', field: 'driver', value: 'postgres',
    });
    expect(next.config.storage).toEqual({ driver: 'postgres' });
  });

  it('marks the owning group dirty', () => {
    const next = reducer(base, {
      type: 'edit/config-field', sectionKey: 'storage', field: 'driver', value: 'postgres',
    });
    expect(groupDirty(next, 'runtime')).toBe(true);
  });
});
```

Note: this test needs `import { groupDirty } from './state';` at the top. If it's not already imported, add it to the existing import block.

- [ ] **Step 2: Run to confirm failure**

```bash
cd web && pnpm test -- state
```

Expected: FAIL — unknown action type / reducer returns state unchanged.

- [ ] **Step 3: Extend the `Action` type and reducer**

Open `web/src/state.ts`. Locate the `Action` type (around line 30). Add a new member:

```ts
export type Action =
  // ...existing members...
  | { type: 'shell/toggleGroup'; group: GroupId }
  | { type: 'edit/config-field'; sectionKey: string; field: string; value: unknown };
```

Add to the reducer switch (before the closing `}`):

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

- [ ] **Step 4: Run the tests to confirm they pass**

```bash
cd web && pnpm test -- state
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/state.ts web/src/state.test.ts
git commit -m "feat(web/state): edit/config-field action for section editors"
```

---

## Task 10: `totalDirtyCount` covers non-gateway groups

**Files:**
- Modify: `web/src/state.ts:246-251` (`totalDirtyCount`)
- Modify: `web/src/state.test.ts`

**Why:** TopBar's `Save · N changes` badge must reflect dirty non-gateway sections.

- [ ] **Step 1: Write the failing test**

Append to `web/src/state.test.ts`:

```ts
describe('totalDirtyCount with non-gateway edits', () => {
  it('returns 1 when only a section is dirty', () => {
    const s: AppState = {
      ...initialState,
      status: 'ready',
      config: { storage: { driver: 'postgres' } },
      originalConfig: { storage: { driver: 'sqlite' } },
    };
    expect(totalDirtyCount(s)).toBe(1);
  });

  it('sums gateway dirty + non-gateway dirty groups', () => {
    const s: AppState = {
      ...initialState,
      status: 'ready',
      config: {
        storage: { driver: 'postgres' },
        gateway: { platforms: { tg: { enabled: true, type: 'telegram', options: { token: 'new' } } } },
      },
      originalConfig: {
        storage: { driver: 'sqlite' },
        gateway: { platforms: { tg: { enabled: true, type: 'telegram', options: { token: 'old' } } } },
      },
    };
    expect(totalDirtyCount(s)).toBe(2);
  });
});
```

Note: ensure `totalDirtyCount` is imported in the test file.

- [ ] **Step 2: Run to confirm failure**

```bash
cd web && pnpm test -- state
```

Expected: the new tests FAIL (current `totalDirtyCount` returns only `dirtyCount`).

- [ ] **Step 3: Update `totalDirtyCount`**

Open `web/src/state.ts`. Replace the existing function body:

```ts
/** totalDirtyCount returns how many units have unsaved changes: the
 *  per-instance gateway diff count plus one per dirty non-gateway group. */
export function totalDirtyCount(state: AppState): number {
  let n = dirtyCount(state);
  for (const g of GROUPS) {
    if (g.id === 'gateway') continue;
    if (groupDirty(state, g.id)) n++;
  }
  return n;
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd web && pnpm test -- state
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/state.ts web/src/state.test.ts
git commit -m "feat(web/state): totalDirtyCount sums dirty non-gateway groups"
```

---

## Task 11: `AppState.configSections` + boot parallel fetch

**Files:**
- Modify: `web/src/api/client.ts`
- Modify: `web/src/state.ts`
- Modify: `web/src/state.test.ts`
- Modify: `web/src/App.tsx:28-55`

**Why:** Loads section schemas alongside platform schemas at boot, stores them in reducer state for later consumption by the renderer.

- [ ] **Step 1: Extend the Action type and initial state**

Open `web/src/state.ts`. Add the import:

```ts
import type { Config, PlatformInstance, SchemaDescriptor, ConfigSection } from './api/schemas';
```

Extend `AppState`:

```ts
export interface AppState {
  status: Status;
  descriptors: SchemaDescriptor[];
  configSections: ConfigSection[];
  config: Config;
  originalConfig: Config;
  selectedKey: string | null;
  flash: Flash | null;
  shell: ShellSliceState;
}
```

Extend the `boot/loaded` action variant:

```ts
| { type: 'boot/loaded'; descriptors: SchemaDescriptor[]; configSections: ConfigSection[]; config: Config }
```

Update `initialState`:

```ts
export const initialState: AppState = {
  status: 'booting',
  descriptors: [],
  configSections: [],
  config: {},
  originalConfig: {},
  selectedKey: null,
  flash: null,
  shell: {
    activeGroup: null,
    activeSubKey: null,
    expandedGroups: loadExpandedGroups(),
  },
};
```

Update the reducer case:

```ts
case 'boot/loaded':
  return {
    ...state,
    status: 'ready',
    descriptors: action.descriptors,
    configSections: action.configSections,
    config: action.config,
    originalConfig: action.config,
  };
```

- [ ] **Step 2: Update the boot dispatch in `App.tsx`**

Open `web/src/App.tsx`. Add the import:

```ts
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  ConfigSchemaResponseSchema,
  PlatformsSchemaResponseSchema,
} from './api/schemas';
```

Replace the boot `useEffect` body (lines 29–55) with:

```ts
  useEffect(() => {
    const ctrl = new AbortController();
    (async () => {
      try {
        const [schema, cfgSchema, cfg] = await Promise.all([
          apiFetch('/api/platforms/schema', {
            schema: PlatformsSchemaResponseSchema,
            signal: ctrl.signal,
          }),
          apiFetch('/api/config/schema', {
            schema: ConfigSchemaResponseSchema,
            signal: ctrl.signal,
          }),
          apiFetch('/api/config', {
            schema: ConfigResponseSchema,
            signal: ctrl.signal,
          }),
        ]);
        dispatch({
          type: 'boot/loaded',
          descriptors: schema.descriptors,
          configSections: cfgSchema.sections,
          config: cfg.config,
        });
      } catch (err) {
        if (ctrl.signal.aborted) return;
        const msg = err instanceof Error ? err.message : 'boot failed';
        dispatch({ type: 'boot/failed', error: msg });
      }
    })();
    return () => ctrl.abort();
  }, []);
```

- [ ] **Step 3: Extend the state test**

Append to `web/src/state.test.ts`:

```ts
describe('boot/loaded carries configSections', () => {
  it('stores sections alongside descriptors and config', () => {
    const next = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: [],
      configSections: [
        { key: 'storage', label: 'Storage', group_id: 'runtime', fields: [] },
      ],
      config: {},
    });
    expect(next.status).toBe('ready');
    expect(next.configSections).toHaveLength(1);
    expect(next.configSections[0].key).toBe('storage');
  });
});
```

- [ ] **Step 4: Run type-check + tests**

```bash
cd web && pnpm type-check && pnpm test -- state
```

Expected: both PASS. If `ConfigSection` or `ConfigSchemaResponseSchema` isn't exported, Task 8's commit is missing — fix by re-running `pnpm install` or verifying the schemas.ts diff.

- [ ] **Step 5: Commit**

```bash
git add web/src/state.ts web/src/state.test.ts web/src/App.tsx
git commit -m "feat(web/state): configSections field + parallel boot fetch"
```

---

## Task 12: `FloatInput` field component

**Files:**
- Create: `web/src/components/fields/FloatInput.tsx`

**Why:** `FieldFloat` needs its own component so the step-any behavior is localized. Copy `NumberInput` verbatim plus `step="any"`.

- [ ] **Step 1: Write the component**

Write `web/src/components/fields/FloatInput.tsx`:

```tsx
import styles from './fields.module.css';
import type { FieldProps } from './TextInput';

export default function FloatInput({ field, value, onChange }: FieldProps) {
  return (
    <label className={styles.row}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <input
        type="number"
        step="any"
        className={`${styles.input} ${styles.number}`}
        value={value}
        placeholder={field.default !== undefined ? String(field.default) : undefined}
        onChange={e => onChange(e.currentTarget.value)}
      />
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
```

- [ ] **Step 2: Verify type-check**

```bash
cd web && pnpm type-check
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/fields/FloatInput.tsx
git commit -m "feat(web/fields): FloatInput — float-valued NumberInput variant"
```

(No unit test — component has no behavior beyond `NumberInput`; the `ConfigSection` test covers its dispatch path. Storage has no FieldFloat, so this file lands unused until stage 3 adds Agent.)

---

## Task 13: `SecretInput.disableReveal` prop

**Files:**
- Modify: `web/src/components/fields/SecretInput.tsx`
- Create: `web/src/components/fields/SecretInput.test.tsx`

**Why:** Section secrets have no per-instance reveal endpoint in Stage 2. The Show button must be disabled with a clear tooltip; the rest of the input (including empty-field-preserves-saved-value behavior on the server side) still works.

- [ ] **Step 1: Write the failing test**

Write `web/src/components/fields/SecretInput.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import SecretInput from './SecretInput';
import type { SchemaField } from '../../api/schemas';

const secretField: SchemaField = {
  name: 'postgres_url',
  label: 'Postgres URL',
  kind: 'secret',
};

describe('SecretInput disableReveal', () => {
  it('disables the Show button and explains why', () => {
    render(
      <SecretInput
        field={secretField}
        value=""
        instanceKey=""
        dirty={false}
        disableReveal
        onChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute('title', 'Reveal not supported for this field (stage 2)');
  });

  it('leaves the Show button enabled when disableReveal is not set', () => {
    render(
      <SecretInput
        field={secretField}
        value=""
        instanceKey="tg_main"
        dirty={false}
        onChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).not.toBeDisabled();
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd web && pnpm test -- SecretInput
```

Expected: FAIL (prop doesn't exist, tooltip missing).

- [ ] **Step 3: Add the prop**

Open `web/src/components/fields/SecretInput.tsx`. Update `SecretInputProps`:

```ts
export interface SecretInputProps {
  field: SchemaField;
  value: string;
  instanceKey: string;
  dirty: boolean;
  disableReveal?: boolean;
  onChange: (value: string) => void;
}
```

Update the destructure and the button:

```tsx
export default function SecretInput({
  field,
  value,
  instanceKey,
  dirty,
  disableReveal,
  onChange,
}: SecretInputProps) {
  // ...existing state...

  const showDisabled = busy || dirty || Boolean(disableReveal);
  const showTitle = disableReveal
    ? 'Reveal not supported for this field (stage 2)'
    : dirty
      ? 'Save changes before revealing the stored value'
      : undefined;

  return (
    <label className={styles.wrap}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <span className={styles.inputRow}>
        <input
          type={revealed ? 'text' : 'password'}
          className={styles.input}
          value={value}
          placeholder="•••"
          onChange={e => {
            setRevealed(false);
            onChange(e.currentTarget.value);
          }}
        />
        <button
          type="button"
          className={styles.revealBtn}
          onClick={onToggle}
          disabled={showDisabled}
          title={showTitle}
        >
          {busy ? '…' : revealed ? 'Hide' : 'Show'}
        </button>
      </span>
      {err && <span className={styles.error}>{err}</span>}
      {field.help && !err && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
```

Leave the `onToggle` implementation unchanged — the button being disabled short-circuits the reveal fetch.

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm test -- SecretInput
```

Expected: PASS. The existing `FieldList` test (which doesn't set `disableReveal`) still passes because the prop is optional.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/fields/SecretInput.tsx web/src/components/fields/SecretInput.test.tsx
git commit -m "feat(web/fields): SecretInput disableReveal prop for section secrets"
```

---

## Task 14: `ConfigSection` generic renderer

**Files:**
- Create: `web/src/components/ConfigSection.tsx`
- Create: `web/src/components/ConfigSection.module.css`
- Create: `web/src/components/ConfigSection.test.tsx`

**Why:** The heart of Stage 2. Takes a descriptor + current/original values; renders only visible fields; dispatches through `onFieldChange`.

- [ ] **Step 1: Write the failing test**

Write `web/src/components/ConfigSection.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ConfigSection from './ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../api/schemas';

const storage: ConfigSectionT = {
  key: 'storage',
  label: 'Storage',
  summary: 'Where hermind keeps data.',
  group_id: 'runtime',
  fields: [
    { name: 'driver', label: 'Driver', kind: 'enum',
      required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
    { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
      visible_when: { field: 'driver', equals: 'sqlite' } },
    { name: 'postgres_url', label: 'Postgres URL', kind: 'secret',
      visible_when: { field: 'driver', equals: 'postgres' } },
  ],
};

describe('ConfigSection', () => {
  it('renders fields whose visible_when matches', () => {
    render(
      <ConfigSection
        section={storage}
        value={{ driver: 'sqlite', sqlite_path: '/var/db/x' }}
        originalValue={{ driver: 'sqlite', sqlite_path: '/var/db/x' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/driver/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/postgres url/i)).not.toBeInTheDocument();
  });

  it('flips visible fields when the discriminator changes', () => {
    const { rerender } = render(
      <ConfigSection
        section={storage}
        value={{ driver: 'sqlite' }}
        originalValue={{ driver: 'sqlite' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();

    rerender(
      <ConfigSection
        section={storage}
        value={{ driver: 'postgres' }}
        originalValue={{ driver: 'sqlite' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText(/sqlite path/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText(/postgres url/i)).toBeInTheDocument();
  });

  it('dispatches onFieldChange with field name + value', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();
    render(
      <ConfigSection
        section={storage}
        value={{ driver: 'sqlite', sqlite_path: '' }}
        originalValue={{ driver: 'sqlite', sqlite_path: '' }}
        onFieldChange={onFieldChange}
      />,
    );
    const input = screen.getByLabelText(/sqlite path/i);
    await user.type(input, '/tmp/x.sqlite');
    // TextInput fires onChange per keystroke; assert last call.
    const calls = onFieldChange.mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    expect(calls[calls.length - 1][0]).toBe('sqlite_path');
    expect(calls[calls.length - 1][1]).toBe('/tmp/x.sqlite');
  });

  it('disables the Show button on secret fields with an explanatory tooltip', () => {
    render(
      <ConfigSection
        section={storage}
        value={{ driver: 'postgres', postgres_url: '' }}
        originalValue={{ driver: 'postgres', postgres_url: '' }}
        onFieldChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute('title', 'Reveal not supported for this field (stage 2)');
  });
});
```

- [ ] **Step 2: Run to confirm it fails**

```bash
cd web && pnpm test -- ConfigSection
```

Expected: FAIL (component doesn't exist).

- [ ] **Step 3: Implement the renderer**

Write `web/src/components/ConfigSection.module.css`:

```css
.section {
  padding: 16px 24px;
  max-width: 640px;
}

.title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 4px 0;
}

.summary {
  color: var(--text-muted, #8b949e);
  font-size: 13px;
  margin: 0 0 16px 0;
}
```

Write `web/src/components/ConfigSection.tsx`:

```tsx
import styles from './ConfigSection.module.css';
import type { ConfigField, ConfigSection as ConfigSectionT } from '../api/schemas';
import TextInput from './fields/TextInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';
import SecretInput from './fields/SecretInput';
import FloatInput from './fields/FloatInput';

export interface ConfigSectionProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onFieldChange: (name: string, value: unknown) => void;
}

export default function ConfigSection({
  section,
  value,
  originalValue,
  onFieldChange,
}: ConfigSectionProps) {
  return (
    <section className={styles.section} aria-label={section.label}>
      <h2 className={styles.title}>{section.label}</h2>
      {section.summary && <p className={styles.summary}>{section.summary}</p>}
      {section.fields.map(f => {
        if (!isVisible(f, value)) return null;
        const current = asString(value[f.name]);
        const original = asString(originalValue[f.name]);
        const onChange = (v: string) => onFieldChange(f.name, v);
        switch (f.kind) {
          case 'int':
            return <NumberInput key={f.name} field={f} value={current} onChange={onChange} />;
          case 'float':
            return <FloatInput key={f.name} field={f} value={current} onChange={onChange} />;
          case 'bool':
            return <BoolToggle key={f.name} field={f} value={current} onChange={onChange} />;
          case 'enum':
            return <EnumSelect key={f.name} field={f} value={current} onChange={onChange} />;
          case 'secret':
            return (
              <SecretInput
                key={f.name}
                field={f}
                value={current}
                instanceKey=""
                dirty={current !== original}
                disableReveal
                onChange={onChange}
              />
            );
          case 'string':
          default:
            return <TextInput key={f.name} field={f} value={current} onChange={onChange} />;
        }
      })}
    </section>
  );
}

function isVisible(f: ConfigField, value: Record<string, unknown>): boolean {
  if (!f.visible_when) return true;
  return value[f.visible_when.field] === f.visible_when.equals;
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  if (typeof v === 'string') return v;
  return String(v);
}
```

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm test -- ConfigSection
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ConfigSection.tsx web/src/components/ConfigSection.module.css web/src/components/ConfigSection.test.tsx
git commit -m "feat(web): ConfigSection generic renderer with visible_when"
```

---

## Task 15: `shell/sections.ts` registry

**Files:**
- Create: `web/src/shell/sections.ts`
- Create: `web/src/shell/sections.test.ts`

**Why:** Client-side registry of which groups own which sections + the planned-stage label. Stage 2 registers Storage only; stages 3–7 append rows.

- [ ] **Step 1: Write the failing test**

Write `web/src/shell/sections.test.ts`:

```ts
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

  it('sectionsInGroup returns entries in declaration order', () => {
    const runtime = sectionsInGroup('runtime');
    const keys = runtime.map(s => s.key);
    expect(keys).toContain('storage');
  });

  it('sectionsInGroup returns [] for a group with no registered sections', () => {
    // Memory group has no sections in stage 2; stage 5 adds them.
    expect(sectionsInGroup('memory')).toEqual([]);
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd web && pnpm test -- sections
```

Expected: FAIL (module not found).

- [ ] **Step 3: Create the registry**

Write `web/src/shell/sections.ts`:

```ts
import type { GroupId } from './groups';

export interface SectionDef {
  key: string;
  groupId: GroupId;
  /** Human-readable stage marker used by the Sidebar and ComingSoonPanel. */
  plannedStage: string;
}

export const SECTIONS: readonly SectionDef[] = [
  { key: 'storage', groupId: 'runtime', plannedStage: 'done' },
] as const;

export function sectionsInGroup(id: GroupId): readonly SectionDef[] {
  return SECTIONS.filter(s => s.groupId === id);
}

export function findSection(key: string): SectionDef | undefined {
  return SECTIONS.find(s => s.key === key);
}
```

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm test -- sections
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/sections.ts web/src/shell/sections.test.ts
git commit -m "feat(web/shell): sections registry — Storage → Runtime"
```

---

## Task 16: `SectionList` sidebar component

**Files:**
- Create: `web/src/components/shell/SectionList.tsx`
- Create: `web/src/components/shell/SectionList.module.css`
- Create: `web/src/components/shell/SectionList.test.tsx`

**Why:** The collapsible row body for non-gateway groups. One row per registered section.

- [ ] **Step 1: Write the failing test**

Write `web/src/components/shell/SectionList.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SectionList from './SectionList';
import type { ConfigSection } from '../../api/schemas';

const storageSection: ConfigSection = {
  key: 'storage',
  label: 'Storage',
  group_id: 'runtime',
  fields: [],
};

describe('SectionList', () => {
  it('renders the labels of registered sections for the group', () => {
    render(
      <SectionList
        group="runtime"
        sections={[storageSection]}
        activeSubKey={null}
        onSelect={() => {}}
      />,
    );
    expect(screen.getByRole('button', { name: /storage/i })).toBeInTheDocument();
  });

  it('marks the active subKey', () => {
    render(
      <SectionList
        group="runtime"
        sections={[storageSection]}
        activeSubKey="storage"
        onSelect={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /storage/i });
    expect(btn).toHaveAttribute('aria-current', 'true');
  });

  it('dispatches onSelect with the section key', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <SectionList
        group="runtime"
        sections={[storageSection]}
        activeSubKey={null}
        onSelect={onSelect}
      />,
    );
    await user.click(screen.getByRole('button', { name: /storage/i }));
    expect(onSelect).toHaveBeenCalledWith('storage');
  });

  it('falls back to a "Coming soon" row when no sections are registered under the group', () => {
    render(
      <SectionList
        group="memory"
        sections={[]}
        activeSubKey={null}
        onSelect={() => {}}
      />,
    );
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd web && pnpm test -- SectionList
```

Expected: FAIL.

- [ ] **Step 3: Implement the component**

Write `web/src/components/shell/SectionList.module.css`:

```css
.list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 8px 8px 24px;
}

.row {
  display: flex;
  align-items: center;
  text-align: left;
  padding: 4px 8px;
  border: none;
  background: transparent;
  color: var(--text, #c9d1d9);
  cursor: pointer;
  border-radius: 3px;
  font-size: 13px;
}

.row:hover {
  background: var(--bg-hover, #1b1f27);
}

.active {
  background: var(--bg-active, #1f6feb33);
  color: var(--text-strong, #f0f6fc);
}

.comingSoon {
  padding: 4px 8px 4px 24px;
  font-size: 12px;
  color: var(--text-muted, #8b949e);
  font-style: italic;
}
```

Write `web/src/components/shell/SectionList.tsx`:

```tsx
import styles from './SectionList.module.css';
import { findGroup, type GroupId } from '../../shell/groups';
import type { ConfigSection } from '../../api/schemas';

export interface SectionListProps {
  group: GroupId;
  sections: readonly ConfigSection[];
  activeSubKey: string | null;
  onSelect: (key: string) => void;
}

export default function SectionList({
  group,
  sections,
  activeSubKey,
  onSelect,
}: SectionListProps) {
  if (sections.length === 0) {
    const def = findGroup(group);
    return (
      <div className={styles.comingSoon}>
        Coming soon — stage {def.plannedStage}
      </div>
    );
  }
  return (
    <div className={styles.list}>
      {sections.map(s => {
        const active = activeSubKey === s.key;
        return (
          <button
            key={s.key}
            type="button"
            className={`${styles.row} ${active ? styles.active : ''}`}
            aria-current={active ? 'true' : undefined}
            onClick={() => onSelect(s.key)}
          >
            {s.label}
          </button>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm test -- SectionList
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/shell/SectionList.tsx web/src/components/shell/SectionList.module.css web/src/components/shell/SectionList.test.tsx
git commit -m "feat(web/shell): SectionList — subsection rows for non-gateway groups"
```

---

## Task 17: Wire `SectionList` into `Sidebar`

**Files:**
- Modify: `web/src/components/shell/Sidebar.tsx`
- Modify: `web/src/components/shell/Sidebar.test.tsx`

**Why:** Today Sidebar renders `GatewaySidebar` inside Gateway and nothing inside other groups' `GroupSection`. Stage 2 makes non-gateway groups render `SectionList` populated from `configSections` filtered by group.

- [ ] **Step 1: Write the failing test**

Append to `web/src/components/shell/Sidebar.test.tsx`:

```tsx
// at top:
// import type { ConfigSection } from '../../api/schemas';

describe('Sidebar — non-gateway groups', () => {
  const storageSection: ConfigSection = {
    key: 'storage', label: 'Storage', group_id: 'runtime', fields: [],
  };

  it('renders registered sections inside expanded non-gateway groups', () => {
    render(
      <Sidebar
        activeGroup="runtime"
        activeSubKey={null}
        expandedGroups={new Set(['runtime'])}
        dirtyGroups={new Set()}
        instances={[]}
        selectedKey={null}
        descriptors={[]}
        configSections={[storageSection]}
        dirtyInstanceKeys={new Set()}
        onSelectGroup={() => {}}
        onSelectSub={() => {}}
        onToggleGroup={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByRole('button', { name: /storage/i })).toBeInTheDocument();
  });

  it('shows "Coming soon" in groups with no registered sections', () => {
    render(
      <Sidebar
        activeGroup={null}
        activeSubKey={null}
        expandedGroups={new Set(['memory'])}
        dirtyGroups={new Set()}
        instances={[]}
        selectedKey={null}
        descriptors={[]}
        configSections={[]}
        dirtyInstanceKeys={new Set()}
        onSelectGroup={() => {}}
        onSelectSub={() => {}}
        onToggleGroup={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByText(/coming soon — stage 5/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd web && pnpm test -- Sidebar
```

Expected: FAIL — `configSections` prop doesn't exist on `Sidebar`.

- [ ] **Step 3: Update `Sidebar`**

Open `web/src/components/shell/Sidebar.tsx`. Replace the file with:

```tsx
import styles from './Sidebar.module.css';
import { GROUPS, type GroupId } from '../../shell/groups';
import GroupSection from './GroupSection';
import GatewaySidebar from '../groups/gateway/GatewaySidebar';
import SectionList from './SectionList';
import type { ConfigSection, SchemaDescriptor } from '../../api/schemas';

export interface SidebarProps {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  expandedGroups: Set<GroupId>;
  dirtyGroups: Set<GroupId>;
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  configSections: ConfigSection[];
  dirtyInstanceKeys: Set<string>;
  onSelectGroup: (id: GroupId) => void;
  onSelectSub: (key: string) => void;
  onToggleGroup: (id: GroupId) => void;
  onNewInstance: () => void;
}

export default function Sidebar(props: SidebarProps) {
  return (
    <aside className={styles.sidebar} aria-label="Configuration groups">
      {GROUPS.map(g => {
        const body =
          g.id === 'gateway' ? (
            <GatewaySidebar
              instances={props.instances}
              selectedKey={props.selectedKey}
              descriptors={props.descriptors}
              dirtyKeys={props.dirtyInstanceKeys}
              onSelect={props.onSelectSub}
              onNewInstance={props.onNewInstance}
            />
          ) : (
            <SectionList
              group={g.id}
              sections={props.configSections.filter(s => s.group_id === g.id)}
              activeSubKey={props.activeGroup === g.id ? props.activeSubKey : null}
              onSelect={key => {
                props.onSelectGroup(g.id);
                props.onSelectSub(key);
              }}
            />
          );
        return (
          <GroupSection
            key={g.id}
            group={g.id}
            expanded={props.expandedGroups.has(g.id)}
            active={props.activeGroup === g.id}
            dirty={props.dirtyGroups.has(g.id)}
            onToggle={() => props.onToggleGroup(g.id)}
            onSelectGroup={props.onSelectGroup}
          >
            {body}
          </GroupSection>
        );
      })}
    </aside>
  );
}
```

- [ ] **Step 4: Fix the Sidebar test's existing cases**

Open `web/src/components/shell/Sidebar.test.tsx`. Every existing `render(<Sidebar … />)` call needs a `configSections={[]}` prop (TypeScript will refuse without it). Walk through each case and add the prop.

- [ ] **Step 5: Run tests + type-check**

```bash
cd web && pnpm type-check && pnpm test -- Sidebar
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/shell/Sidebar.tsx web/src/components/shell/Sidebar.test.tsx
git commit -m "feat(web/shell): Sidebar renders SectionList for non-gateway groups"
```

---

## Task 18: Wire `ContentPanel` routing for Storage

**Files:**
- Modify: `web/src/components/shell/ContentPanel.tsx`
- Modify: `web/src/components/shell/ContentPanel.test.tsx`

**Why:** Route `#runtime/storage` to the new `ConfigSection`; keep `ComingSoonPanel` as the default for everything else.

- [ ] **Step 1: Write the failing test**

Append to `web/src/components/shell/ContentPanel.test.tsx`:

```tsx
// at top:
// import type { ConfigSection } from '../../api/schemas';

describe('ContentPanel — non-gateway section routing', () => {
  const storageSection: ConfigSection = {
    key: 'storage',
    label: 'Storage',
    summary: 'Where hermind keeps data.',
    group_id: 'runtime',
    fields: [
      { name: 'driver', label: 'Driver', kind: 'enum',
        required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
      { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
        visible_when: { field: 'driver', equals: 'sqlite' } },
    ],
  };

  const baseProps = {
    activeGroup: 'runtime' as const,
    activeSubKey: 'storage',
    config: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x' } },
    originalConfig: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x' } },
    configSections: [storageSection],
    selectedKey: null,
    instance: null,
    originalInstance: null,
    descriptor: null,
    dirtyGateway: false,
    busy: false,
    onField: () => {},
    onToggleEnabled: () => {},
    onDelete: () => {},
    onApply: () => {},
    onSelectGroup: () => {},
    onConfigField: () => {},
  };

  it('renders the ConfigSection for runtime/storage', () => {
    render(<ContentPanel {...baseProps} />);
    expect(screen.getByLabelText(/driver/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();
  });

  it('falls back to ComingSoonPanel when subKey does not match a registered section', () => {
    render(
      <ContentPanel {...baseProps} activeSubKey="somethingelse" />,
    );
    // ComingSoonPanel shows "<GroupLabel> — coming soon"
    expect(screen.getByText(/runtime — coming soon/i)).toBeInTheDocument();
  });

  it('falls back to ComingSoonPanel when subKey is null', () => {
    render(
      <ContentPanel {...baseProps} activeSubKey={null} />,
    );
    expect(screen.getByText(/runtime — coming soon/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd web && pnpm test -- ContentPanel
```

Expected: FAIL (props `configSections`, `originalConfig`, `onConfigField` missing).

- [ ] **Step 3: Extend `ContentPanel`**

Open `web/src/components/shell/ContentPanel.tsx`. Replace with:

```tsx
import type { Config, ConfigSection as ConfigSectionT, PlatformInstance, SchemaDescriptor } from '../../api/schemas';
import { type GroupId } from '../../shell/groups';
import ComingSoonPanel from './ComingSoonPanel';
import EmptyState from './EmptyState';
import GatewayPanel from '../groups/gateway/GatewayPanel';
import ConfigSection from '../ConfigSection';

export interface ContentPanelProps {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  config: Config;
  originalConfig: Config;
  configSections: ConfigSectionT[];
  selectedKey: string | null;
  instance: PlatformInstance | null;
  originalInstance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  dirtyGateway: boolean;
  busy: boolean;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
  onApply: () => void;
  onSelectGroup: (id: GroupId) => void;
  onConfigField: (sectionKey: string, field: string, value: unknown) => void;
}

export default function ContentPanel(props: ContentPanelProps) {
  if (props.activeGroup === null) {
    return <EmptyState onSelectGroup={props.onSelectGroup} />;
  }
  if (props.activeGroup === 'gateway') {
    return (
      <GatewayPanel
        selectedKey={props.selectedKey}
        instance={props.instance}
        originalInstance={props.originalInstance}
        descriptor={props.descriptor}
        dirty={props.dirtyGateway}
        busy={props.busy}
        onField={props.onField}
        onToggleEnabled={props.onToggleEnabled}
        onDelete={props.onDelete}
        onApply={props.onApply}
      />
    );
  }
  if (props.activeSubKey) {
    const section = props.configSections.find(
      s => s.key === props.activeSubKey && s.group_id === props.activeGroup,
    );
    if (section) {
      const value = (props.config as Record<string, unknown>)[section.key] as
        | Record<string, unknown>
        | undefined;
      const original = (props.originalConfig as Record<string, unknown>)[section.key] as
        | Record<string, unknown>
        | undefined;
      return (
        <ConfigSection
          section={section}
          value={value ?? {}}
          originalValue={original ?? {}}
          onFieldChange={(field, v) => props.onConfigField(section.key, field, v)}
        />
      );
    }
  }
  return <ComingSoonPanel group={props.activeGroup} config={props.config} />;
}
```

Update the existing test cases in `ContentPanel.test.tsx` that render `<ContentPanel>` to include the new required props (`activeSubKey`, `originalConfig`, `configSections`, `onConfigField`). Use empty defaults where they weren't previously set.

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm type-check && pnpm test -- ContentPanel
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/shell/ContentPanel.tsx web/src/components/shell/ContentPanel.test.tsx
git commit -m "feat(web/shell): ContentPanel routes <group>/<sub> to ConfigSection"
```

---

## Task 19: Wire `App.tsx` to the new props

**Files:**
- Modify: `web/src/App.tsx`
- Create: `web/src/App.test.tsx`

**Why:** Thread `configSections`, `originalConfig`, and `onConfigField` into `Sidebar` and `ContentPanel`. Write a small integration test that proves the happy path end-to-end.

- [ ] **Step 1: Lift the gateway-only guard on sub-key dispatch**

Open `web/src/App.tsx`. Inside the `useEffect` that resolves the initial hash (around line 68), locate:

```ts
if (parsed.group) {
  dispatch({ type: 'shell/selectGroup', group: parsed.group });
  if (parsed.sub && parsed.group === 'gateway') {
    dispatch({ type: 'shell/selectSub', key: parsed.sub });
  }
}
```

Replace with:

```ts
if (parsed.group) {
  dispatch({ type: 'shell/selectGroup', group: parsed.group });
  if (parsed.sub) {
    dispatch({ type: 'shell/selectSub', key: parsed.sub });
  }
}
```

The previous guard was Stage-1 specific — only Gateway had sub-keys. Stage 2's `#runtime/storage` also needs the sub-key dispatched. The reducer's `shell/selectSub` case already only mirrors to `selectedKey` when `activeGroup === 'gateway'`, so non-gateway sub-keys don't pollute the legacy IM path.

- [ ] **Step 2: Update `<Sidebar … />` props wiring**

Locate the `<Sidebar … />` call and add:

```tsx
configSections={state.configSections}
```

Immediately below `descriptors={state.descriptors}`.

Locate the `<ContentPanel … />` call. Add the new props and rename the existing ones where the interface changed:

```tsx
<ContentPanel
  activeGroup={state.shell.activeGroup}
  activeSubKey={state.shell.activeSubKey}
  config={state.config}
  originalConfig={state.originalConfig}
  configSections={state.configSections}
  selectedKey={selectedKey}
  instance={selectedInstance}
  originalInstance={selectedOriginal}
  descriptor={selectedDescriptor}
  dirtyGateway={dirtyGroupIds.has('gateway')}
  busy={busy}
  onField={(field, value) =>
    selectedKey && dispatch({ type: 'edit/field', key: selectedKey, field, value })
  }
  onToggleEnabled={enabled =>
    selectedKey && dispatch({ type: 'edit/enabled', key: selectedKey, enabled })
  }
  onDelete={() => selectedKey && dispatch({ type: 'instance/delete', key: selectedKey })}
  onApply={onApplyGateway}
  onSelectGroup={(id: GroupId) => dispatch({ type: 'shell/selectGroup', group: id })}
  onConfigField={(sectionKey, field, value) =>
    dispatch({ type: 'edit/config-field', sectionKey, field, value })
  }
/>
```

- [ ] **Step 3: Write the integration test**

Write `web/src/App.test.tsx`:

```tsx
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

function mockBoot() {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : (input as Request).url;
    if (url.endsWith('/api/platforms/schema')) {
      return jsonResponse({ descriptors: [] });
    }
    if (url.endsWith('/api/config/schema')) {
      return jsonResponse({
        sections: [
          {
            key: 'storage',
            label: 'Storage',
            summary: 'Where hermind keeps data.',
            group_id: 'runtime',
            fields: [
              { name: 'driver', label: 'Driver', kind: 'enum',
                required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
              { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
                visible_when: { field: 'driver', equals: 'sqlite' } },
              { name: 'postgres_url', label: 'Postgres URL', kind: 'secret',
                visible_when: { field: 'driver', equals: 'postgres' } },
            ],
          },
        ],
      });
    }
    if (url.endsWith('/api/config') && (!input || !(input as Request).method || (input as Request).method === 'GET')) {
      return jsonResponse({
        config: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x' } },
      });
    }
    return jsonResponse({}, 200);
  });
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

describe('App integration — storage vertical slice', () => {
  beforeEach(() => {
    window.location.hash = '#runtime/storage';
    mockBoot();
  });
  afterEach(() => {
    vi.restoreAllMocks();
    window.location.hash = '';
  });

  it('renders the storage editor on #runtime/storage and flips fields on driver change', async () => {
    const user = userEvent.setup();
    render(<App />);

    // Boot → sidebar + storage fields appear.
    await waitFor(() => {
      expect(screen.getByLabelText(/driver/i)).toBeInTheDocument();
    });
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/postgres url/i)).not.toBeInTheDocument();

    // Flip driver → postgres.
    const driver = screen.getByLabelText(/driver/i) as HTMLSelectElement;
    await user.selectOptions(driver, 'postgres');

    expect(screen.queryByLabelText(/sqlite path/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText(/postgres url/i)).toBeInTheDocument();

    // TopBar badge shows one change.
    expect(screen.getByRole('button', { name: /save · 1 changes/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run tests + type-check**

```bash
cd web && pnpm type-check && pnpm test -- App
```

Expected: PASS. If the Save button's label format doesn't include "Save · 1 changes" verbatim, inspect `TopBar.tsx` and adjust the regex to the actual format used.

- [ ] **Step 5: Commit**

```bash
git add web/src/App.tsx web/src/App.test.tsx
git commit -m "feat(web): App wires Storage through Sidebar + ContentPanel end-to-end"
```

---

## Task 20: Rebuild `api/webroot/`, run full check, update smoke doc

**Files:**
- Modify: `api/webroot/` (regenerated bundle)
- Modify: `docs/smoke/web-config.md`

**Why:** CI asserts `api/webroot/` matches the current Vite build, and the smoke doc gains a Stage-2 section.

- [ ] **Step 1: Rebuild the web bundle and sync `api/webroot/`**

```bash
make web
```

Expected: `pnpm build` succeeds; `api/webroot/` is refreshed.

- [ ] **Step 2: Run the CI gate**

```bash
make web-check
```

Expected: `type-check`, `pnpm test`, `pnpm lint`, `pnpm build`, the `api/webroot/` sync assertion all PASS. Zero lint warnings.

- [ ] **Step 3: Run the full Go test suite**

```bash
go test ./config/descriptor/... ./api/...
```

Expected: PASS.

- [ ] **Step 4: Append a Stage-2 section to `docs/smoke/web-config.md`**

Open `docs/smoke/web-config.md` and append the following section at the end of the file (or merge into the existing "Stage 1 · Shell rewrite" heading if the file uses one):

```markdown
## Stage 2 · Schema infrastructure (Storage)

- Visiting `#runtime/storage` renders the Storage editor: a Driver enum select, plus either a SQLite path field (driver=sqlite) or a Postgres URL field (driver=postgres).
- Changing the Driver value swaps which secondary field is visible; the hidden field's value is not submitted until re-shown.
- The Postgres URL field is a secret: the Show button is disabled with tooltip "Reveal not supported for this field (stage 2)".
- `GET /api/config/schema` returns a `sections` array that includes `storage` with its three fields and two visible_when predicates.
- `GET /api/config` with `storage.driver = postgres` returns `postgres_url` blanked.
- `PUT /api/config` with `storage.postgres_url = ""` preserves the stored URL (round-trip mirror of platform-secret behavior).
- Editing any storage field marks the Runtime group dirty (sidebar dot + TopBar `Save · N changes`). Save flushes to disk; the YAML reflects the new driver and the appropriate path/URL field.
- Routing: the Runtime group in the sidebar lists one entry — Storage. Other non-gateway groups show "Coming soon — stage N" inside their collapsible rows.
```

- [ ] **Step 5: Commit**

```bash
git add api/webroot/ docs/smoke/web-config.md
git commit -m "chore(web): rebuild api/webroot + smoke doc for schema infra"
```

---

## Completion checklist

Before calling Stage 2 done, verify:

- [ ] `go test ./config/descriptor/... ./api/...` — PASS
- [ ] `cd web && pnpm test` — PASS (all existing + new specs)
- [ ] `cd web && pnpm type-check` — PASS
- [ ] `cd web && pnpm lint` — zero warnings
- [ ] `make web-check` — PASS (`api/webroot/` sync assertion + everything above)
- [ ] Manual smoke: `hermind web` → navigate to `#runtime/storage` → observe the Driver enum + conditional path/URL field → flip driver → Save → grep `storage:` in the written YAML to confirm the change landed
- [ ] `docs/smoke/web-config.md` has the new Stage 2 section
- [ ] `git status --short` — empty modulo other in-progress stages

Once all boxes are checked, Stage 2 is complete. Stage 3 (simple sections — Logging, Metrics, Tracing, Agent, Terminal, Model) can begin by adding descriptor files under `config/descriptor/` and appending entries to `web/src/shell/sections.ts`; no further infrastructure work required.
