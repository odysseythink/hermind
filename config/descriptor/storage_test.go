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
