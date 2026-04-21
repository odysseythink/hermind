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

func TestSkillsSectionHasDisabledMultiSelectField(t *testing.T) {
	s, _ := Get("skills")
	if len(s.Fields) != 1 {
		t.Fatalf("expected exactly 1 field, got %d: %+v", len(s.Fields), s.Fields)
	}
	f := s.Fields[0]
	if f.Name != "disabled" {
		t.Errorf("field name = %q, want %q", f.Name, "disabled")
	}
	if f.Kind != FieldMultiSelect {
		t.Errorf("field kind = %s, want multiselect", f.Kind)
	}
	if f.Required {
		t.Errorf("field.Required = true, want false (empty disabled list means all enabled)")
	}
	if f.Help == "" {
		t.Errorf("field.Help is empty; users need a hint about what this does")
	}
	// Enum is left empty at descriptor registration time; handler enriches
	// it from the skills loader before emitting the schema DTO.
	if len(f.Enum) != 0 {
		t.Errorf("field.Enum should be empty at registration time (got %v); "+
			"runtime enrichment via handlers_config_schema.go supplies choices", f.Enum)
	}
}
