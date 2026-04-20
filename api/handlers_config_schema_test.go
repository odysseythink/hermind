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
