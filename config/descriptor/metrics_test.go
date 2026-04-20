package descriptor

import "testing"

func TestMetricsSectionRegistered(t *testing.T) {
	s, ok := Get("metrics")
	if !ok {
		t.Fatal(`Get("metrics") returned ok=false — did metrics.go init() register?`)
	}
	if s.GroupID != "observability" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "observability")
	}
	if len(s.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1", len(s.Fields))
	}
	f := s.Fields[0]
	if f.Name != "addr" {
		t.Errorf("Fields[0].Name = %q, want %q", f.Name, "addr")
	}
	if f.Kind != FieldString {
		t.Errorf("Fields[0].Kind = %s, want string", f.Kind)
	}
	if f.Required {
		t.Error("addr.Required = true, want false (empty disables metrics)")
	}
}
