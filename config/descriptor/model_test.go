package descriptor

import "testing"

func TestModelSectionRegistered(t *testing.T) {
	s, ok := Get("model")
	if !ok {
		t.Fatal(`Get("model") returned ok=false — did model.go init() register?`)
	}
	if s.GroupID != "models" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "models")
	}
	if s.Shape != ShapeScalar {
		t.Errorf("Shape = %v, want ShapeScalar", s.Shape)
	}
	if len(s.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1 (ShapeScalar invariant)", len(s.Fields))
	}
	f := s.Fields[0]
	if f.Name != "model" {
		t.Errorf("Fields[0].Name = %q, want %q", f.Name, "model")
	}
	if f.Kind != FieldString {
		t.Errorf("Fields[0].Kind = %s, want string", f.Kind)
	}
	if !f.Required {
		t.Error("Fields[0].Required = false, want true (default model is not optional)")
	}
	if f.Help == "" {
		t.Error("Fields[0].Help is empty — the field needs a hint about the provider-qualified format")
	}
}
