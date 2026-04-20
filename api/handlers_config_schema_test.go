package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
