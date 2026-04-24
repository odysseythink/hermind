package descriptor

import "testing"

func TestGatewaySection(t *testing.T) {
	s, ok := Get("gateway")
	if !ok {
		t.Fatal("gateway section not registered")
	}
	if s.Label != "IM Channels" {
		t.Errorf("Label = %q, want %q", s.Label, "IM Channels")
	}
	if s.Shape != ShapeKeyedMap {
		t.Errorf("Shape = %v, want %v", s.Shape, ShapeKeyedMap)
	}
	if s.Subkey != "platforms" {
		t.Errorf("Subkey = %q, want %q", s.Subkey, "platforms")
	}
	if s.GroupID != "gateway" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "gateway")
	}
	if !s.NoDiscriminator {
		t.Error("NoDiscriminator should be true")
	}
	if len(s.Fields) < 5 {
		t.Fatalf("Fields len = %d, want >= 5", len(s.Fields))
	}
	// First two fields are always type and enabled.
	if s.Fields[0].Name != "type" || s.Fields[0].Kind != FieldEnum {
		t.Errorf("Fields[0] = {%q, %v}, want {type, FieldEnum}", s.Fields[0].Name, s.Fields[0].Kind)
	}
	if s.Fields[1].Name != "enabled" || s.Fields[1].Kind != FieldBool {
		t.Errorf("Fields[1] = {%q, %v}, want {enabled, FieldBool}", s.Fields[1].Name, s.Fields[1].Kind)
	}
	// Verify all per-platform option fields use VisibleWhen.
	for _, f := range s.Fields[2:] {
		if f.VisibleWhen == nil {
			t.Errorf("field %q: per-platform field must have VisibleWhen set", f.Name)
		}
	}
	// At least one secret field must exist.
	var secrets int
	for _, f := range s.Fields {
		if f.Kind == FieldSecret {
			secrets++
		}
	}
	if secrets == 0 {
		t.Error("gateway section should have at least one FieldSecret")
	}
}
