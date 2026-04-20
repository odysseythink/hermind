package descriptor

import "testing"

func TestTracingSectionRegistered(t *testing.T) {
	s, ok := Get("tracing")
	if !ok {
		t.Fatal(`Get("tracing") returned ok=false`)
	}
	if s.GroupID != "observability" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "observability")
	}
	names := map[string]FieldSpec{}
	for _, f := range s.Fields {
		names[f.Name] = f
	}
	enabled, ok := names["enabled"]
	if !ok {
		t.Fatal("missing enabled field")
	}
	if enabled.Kind != FieldBool {
		t.Errorf("enabled.Kind = %s, want bool", enabled.Kind)
	}
	file, ok := names["file"]
	if !ok {
		t.Fatal("missing file field")
	}
	if file.Kind != FieldString {
		t.Errorf("file.Kind = %s, want string", file.Kind)
	}
	if file.VisibleWhen == nil {
		t.Fatal("file.VisibleWhen is nil")
	}
	if file.VisibleWhen.Field != "enabled" {
		t.Errorf("file.VisibleWhen.Field = %q, want \"enabled\"", file.VisibleWhen.Field)
	}
	if file.VisibleWhen.Equals != true {
		t.Errorf("file.VisibleWhen.Equals = %v, want true", file.VisibleWhen.Equals)
	}
}
