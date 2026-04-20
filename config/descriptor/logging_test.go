package descriptor

import "testing"

func TestLoggingSectionRegistered(t *testing.T) {
	s, ok := Get("logging")
	if !ok {
		t.Fatal(`Get("logging") returned ok=false — did logging.go init() register?`)
	}
	if s.GroupID != "observability" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "observability")
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
	if len(s.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1", len(s.Fields))
	}
	f := s.Fields[0]
	if f.Name != "level" {
		t.Errorf("Fields[0].Name = %q, want %q", f.Name, "level")
	}
	if f.Kind != FieldEnum {
		t.Errorf("Fields[0].Kind = %s, want enum", f.Kind)
	}
	want := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	for _, v := range f.Enum {
		delete(want, v)
	}
	if len(want) > 0 {
		t.Errorf("level.Enum missing %v, got %v", want, f.Enum)
	}
	if f.Default != "info" {
		t.Errorf("level.Default = %v, want \"info\"", f.Default)
	}
}
