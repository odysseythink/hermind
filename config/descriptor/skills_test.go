package descriptor

import "testing"

func TestSkillsSectionRegistered(t *testing.T) {
	s, ok := Get("skills")
	if !ok {
		t.Fatalf("Get(\"skills\") returned ok=false — did skills.go init() register?")
	}
	if s.GroupID != "skills" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "skills")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestSkillsSectionFields(t *testing.T) {
	s, _ := Get("skills")
	want := []struct {
		name    string
		kind    FieldKind
		def     any
		hasHelp bool
	}{
		{"disabled", FieldMultiSelect, nil, true},
		{"auto_extract", FieldBool, false, true},
		{"inject_count", FieldInt, 3, true},
		{"generation_half_life", FieldInt, 5, true},
	}
	if len(s.Fields) != len(want) {
		t.Fatalf("field count = %d, want %d: %+v", len(s.Fields), len(want), s.Fields)
	}
	for i, w := range want {
		f := s.Fields[i]
		if f.Name != w.name {
			t.Errorf("field[%d].Name = %q, want %q", i, f.Name, w.name)
		}
		if f.Kind != w.kind {
			t.Errorf("field[%d=%s].Kind = %s, want %s", i, f.Name, f.Kind, w.kind)
		}
		if w.def != nil && f.Default != w.def {
			t.Errorf("field[%d=%s].Default = %v, want %v", i, f.Name, f.Default, w.def)
		}
		if w.hasHelp && f.Help == "" {
			t.Errorf("field[%d=%s].Help is empty", i, f.Name)
		}
	}
	// `disabled` Enum must remain empty at registration time — handler
	// enriches it from the skills loader.
	if len(s.Fields[0].Enum) != 0 {
		t.Errorf("disabled.Enum at registration should be empty, got %v", s.Fields[0].Enum)
	}
}
