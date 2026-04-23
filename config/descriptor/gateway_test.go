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
	if len(s.Fields) != 3 {
		t.Fatalf("Fields len = %d, want 3", len(s.Fields))
	}

	want := []struct {
		name     string
		kind     FieldKind
		required bool
	}{
		{"type", FieldEnum, true},
		{"enabled", FieldBool, false},
		{"options", FieldText, false},
	}
	for i, w := range want {
		got := s.Fields[i]
		if got.Name != w.name {
			t.Errorf("Fields[%d].Name = %q, want %q", i, got.Name, w.name)
		}
		if got.Kind != w.kind {
			t.Errorf("Fields[%d].Kind = %v, want %v", i, got.Kind, w.kind)
		}
		if got.Required != w.required {
			t.Errorf("Fields[%d].Required = %v, want %v", i, got.Required, w.required)
		}
	}
}
