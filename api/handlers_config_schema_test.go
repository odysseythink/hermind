package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/config/descriptor"
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

func TestConfigSchema_OmitsShapeForMapSections(t *testing.T) {
	// ShapeMap (the default zero value) must NOT emit a "shape" key in the
	// JSON. Protects byte-level backwards compat for Stage 2/3 sections.
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
	// Parse into a generic map so we can check for the literal absence of
	// the "shape" key rather than a zero-value string.
	var body struct {
		Sections []map[string]any `json:"sections"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, sec := range body.Sections {
		key, _ := sec["key"].(string)
		if key == "storage" || key == "agent" || key == "terminal" ||
			key == "logging" || key == "metrics" || key == "tracing" {
			if _, present := sec["shape"]; present {
				t.Errorf("section %q: shape key present, want absent for ShapeMap", key)
			}
		}
	}
}

func TestConfigSchema_IncludesStage4aSections(t *testing.T) {
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

	var model, auxiliary *api.ConfigSectionDTO
	for i := range body.Sections {
		switch body.Sections[i].Key {
		case "model":
			model = &body.Sections[i]
		case "auxiliary":
			auxiliary = &body.Sections[i]
		}
	}
	if model == nil {
		t.Fatal("missing section \"model\"")
	}
	if auxiliary == nil {
		t.Fatal("missing section \"auxiliary\"")
	}
	if model.GroupID != "models" {
		t.Errorf("model.group_id = %q, want %q", model.GroupID, "models")
	}
	if model.Shape != "scalar" {
		t.Errorf("model.shape = %q, want %q", model.Shape, "scalar")
	}
	if len(model.Fields) != 1 {
		t.Errorf("model.fields length = %d, want 1", len(model.Fields))
	}
	if auxiliary.GroupID != "runtime" {
		t.Errorf("auxiliary.group_id = %q, want %q", auxiliary.GroupID, "runtime")
	}
	if auxiliary.Shape != "" {
		t.Errorf("auxiliary.shape = %q, want \"\" (map sections omit shape)", auxiliary.Shape)
	}
	// api_key must still be blanked for auxiliary via existing redact plumbing
	var apiKey *api.ConfigFieldDTO
	for i := range auxiliary.Fields {
		if auxiliary.Fields[i].Name == "api_key" {
			apiKey = &auxiliary.Fields[i]
			break
		}
	}
	if apiKey == nil {
		t.Fatal("auxiliary.fields missing api_key")
	}
	if apiKey.Kind != "secret" {
		t.Errorf("auxiliary.api_key.kind = %q, want \"secret\"", apiKey.Kind)
	}
}

func TestConfigSchema_IncludesStage4cSections(t *testing.T) {
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

	var fb *api.ConfigSectionDTO
	for i := range body.Sections {
		if body.Sections[i].Key == "fallback_providers" {
			fb = &body.Sections[i]
			break
		}
	}
	if fb == nil {
		t.Fatal("fallback_providers section missing from /api/config/schema")
	}
	if fb.Shape != "list" {
		t.Errorf("fallback_providers.shape = %q, want %q", fb.Shape, "list")
	}
	if fb.GroupID != "models" {
		t.Errorf("fallback_providers.group_id = %q, want %q", fb.GroupID, "models")
	}
	if len(fb.Fields) != 4 {
		t.Errorf("fallback_providers.fields length = %d, want 4", len(fb.Fields))
	}
	var apiKey *api.ConfigFieldDTO
	for i := range fb.Fields {
		if fb.Fields[i].Name == "api_key" {
			apiKey = &fb.Fields[i]
			break
		}
	}
	if apiKey == nil {
		t.Fatal("fallback_providers.fields missing api_key")
	}
	if apiKey.Kind != "secret" {
		t.Errorf("fallback_providers.api_key.kind = %q, want \"secret\"", apiKey.Kind)
	}
}

func TestConfigSchema_EmitsDatalistSource(t *testing.T) {
	const key = "__test_schema_datalist"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeScalar,
		Fields: []descriptor.FieldSpec{
			{
				Name:  "pick_model",
				Label: "Pick model",
				Kind:  descriptor.FieldString,
				DatalistSource: &descriptor.DatalistSource{
					Section: "providers",
					Field:   "model",
				},
			},
		},
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
		Sections []struct {
			Key    string `json:"key"`
			Fields []struct {
				Name           string         `json:"name"`
				DatalistSource map[string]any `json:"datalist_source"`
			} `json:"fields"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}

	var found bool
	for _, sec := range body.Sections {
		if sec.Key != key {
			continue
		}
		for _, f := range sec.Fields {
			if f.Name != "pick_model" {
				continue
			}
			found = true
			if f.DatalistSource == nil {
				t.Error("datalist_source missing from emitted field")
				return
			}
			if f.DatalistSource["section"] != "providers" {
				t.Errorf("datalist_source.section = %v, want \"providers\"", f.DatalistSource["section"])
			}
			if f.DatalistSource["field"] != "model" {
				t.Errorf("datalist_source.field = %v, want \"model\"", f.DatalistSource["field"])
			}
		}
	}
	if !found {
		t.Fatalf("seeded section %q not present in response", key)
	}
}

func TestConfigSchema_OmitsDatalistSourceByDefault(t *testing.T) {
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	req := httptest.NewRequest("GET", "/api/config/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var body struct {
		Sections []map[string]any `json:"sections"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	for _, sec := range body.Sections {
		if sec["key"] != "storage" {
			continue
		}
		fields, _ := sec["fields"].([]any)
		for _, raw := range fields {
			f, _ := raw.(map[string]any)
			if _, present := f["datalist_source"]; present {
				t.Errorf("storage field %v: datalist_source must be omitted when unset, got %v",
					f["name"], f["datalist_source"])
			}
		}
	}
}

func TestConfigSchema_EmitsListShapeString(t *testing.T) {
	// Seed a ShapeList section directly via Register so this test doesn't
	// depend on Task 4's fallback_providers descriptor having landed.
	const key = "__test_schema_list"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   descriptor.ShapeList,
		Fields: []descriptor.FieldSpec{
			{Name: "provider", Label: "Type", Kind: descriptor.FieldEnum,
				Required: true, Enum: []string{"a", "b"}},
			{Name: "api_key", Label: "API key", Kind: descriptor.FieldSecret},
		},
	})
	// The registry is process-global. The "__" prefix isolates this seed
	// from production sections, and its 2 fields satisfy TestSectionInvariants
	// (1 provider-enum, len>0).

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
			if shape, _ := sec["shape"].(string); shape != "list" {
				t.Errorf("shape = %q, want %q", shape, "list")
			}
		}
	}
	if !found {
		t.Fatalf("seeded section %q not present in response", key)
	}
}

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
	// The registry is process-global and has no public Deregister. We leave
	// the seed in place — its "__" prefix keeps it isolated from production
	// sections and its 2 fields satisfy TestSectionInvariants
	// (1 provider-enum, len>0).

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

func TestConfigSchema_SkillsDisabledEnumFromLoader(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Seed two skills under $HERMIND_HOME/skills/<category>/<name>/SKILL.md.
	seed := func(name string) {
		p := filepath.Join(dir, "skills", "demo", name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: seeded for test\n---\nbody"
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed("alpha")
	seed("beta")

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

	var skills *api.ConfigSectionDTO
	for i := range body.Sections {
		if body.Sections[i].Key == "skills" {
			skills = &body.Sections[i]
			break
		}
	}
	if skills == nil {
		t.Fatalf("missing skills section; got keys = %v", keysOf(body.Sections))
	}
	if skills.GroupID != "skills" {
		t.Errorf("skills.group_id = %q, want \"skills\"", skills.GroupID)
	}
	if len(skills.Fields) != 1 {
		t.Fatalf("skills.fields count = %d, want 1", len(skills.Fields))
	}
	disabled := skills.Fields[0]
	if disabled.Name != "disabled" {
		t.Errorf("field name = %q, want \"disabled\"", disabled.Name)
	}
	if disabled.Kind != "multiselect" {
		t.Errorf("field kind = %q, want \"multiselect\"", disabled.Kind)
	}

	if !reflect.DeepEqual(disabled.Enum, []string{"alpha", "beta"}) {
		t.Errorf("disabled.Enum = %v, want [alpha beta] in sorted order", disabled.Enum)
	}
}

// keysOf returns the Key of each section in order — used to produce
// readable failure messages when a section lookup fails.
func keysOf(sections []api.ConfigSectionDTO) []string {
	out := make([]string, len(sections))
	for i, s := range sections {
		out[i] = s.Key
	}
	return out
}

func TestConfigSchema_EmitsSubkeyAndNoDiscriminator(t *testing.T) {
	const key = "__test_schema_subkey"
	descriptor.Register(descriptor.Section{
		Key:             key,
		Label:           "Subkey probe",
		GroupID:         "runtime",
		Shape:           descriptor.ShapeKeyedMap,
		Subkey:          "servers",
		NoDiscriminator: true,
		Fields: []descriptor.FieldSpec{
			{Name: "command", Label: "Command", Kind: descriptor.FieldString, Required: true},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

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
	var found map[string]any
	for _, sec := range body.Sections {
		if k, _ := sec["key"].(string); k == key {
			found = sec
			break
		}
	}
	if found == nil {
		t.Fatalf("seeded section %q not present in response", key)
	}
	if sk, _ := found["subkey"].(string); sk != "servers" {
		t.Errorf("subkey = %q, want \"servers\"", sk)
	}
	if nd, _ := found["no_discriminator"].(bool); !nd {
		t.Errorf("no_discriminator = %v, want true", found["no_discriminator"])
	}
}

func TestConfigSchema_OmitsSubkeyAndNoDiscriminatorWhenUnset(t *testing.T) {
	const key = "__test_schema_no_subkey"
	descriptor.Register(descriptor.Section{
		Key:     key,
		Label:   "No-subkey probe",
		GroupID: "runtime",
		Shape:   descriptor.ShapeMap,
		Fields: []descriptor.FieldSpec{
			{Name: "f", Label: "F", Kind: descriptor.FieldString},
		},
	})
	t.Cleanup(func() { descriptor.Unregister(key) })

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
	var body struct {
		Sections []map[string]any `json:"sections"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	var found map[string]any
	for _, sec := range body.Sections {
		if k, _ := sec["key"].(string); k == key {
			found = sec
			break
		}
	}
	if found == nil {
		t.Fatalf("seeded section %q not present", key)
	}
	// Keys must be absent (omitempty) when unset, not present with zero values.
	if _, has := found["subkey"]; has {
		t.Errorf("subkey key should be omitted when empty; got %v", found["subkey"])
	}
	if _, has := found["no_discriminator"]; has {
		t.Errorf("no_discriminator key should be omitted when false; got %v", found["no_discriminator"])
	}
}
