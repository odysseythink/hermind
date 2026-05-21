package descriptor

import "testing"

func TestEmbedModelSectionRegistered(t *testing.T) {
	s, ok := Get("embed_model")
	if !ok {
		t.Fatal(`Get("embed_model") returned ok=false — did embed_model.go init() register?`)
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
	if f.Name != "embed_model" {
		t.Errorf("Fields[0].Name = %q, want %q", f.Name, "embed_model")
	}
	if f.Kind != FieldString {
		t.Errorf("Fields[0].Kind = %s, want string", f.Kind)
	}
	if f.Required {
		t.Error("Fields[0].Required = true, want false (embed model has a default)")
	}
	if f.Default != "text-embedding-3-small" {
		t.Errorf("Fields[0].Default = %q, want %q", f.Default, "text-embedding-3-small")
	}
	if f.Help == "" {
		t.Error("Fields[0].Help is empty — the field needs a hint about provider-qualified format")
	}
}
