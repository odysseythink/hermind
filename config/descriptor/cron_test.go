package descriptor

import "testing"

func TestCronSectionRegistered(t *testing.T) {
	s, ok := Get("cron")
	if !ok {
		t.Fatal("Get(\"cron\") returned ok=false — did cron.go init() register?")
	}
	if s.GroupID != "advanced" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "advanced")
	}
	if s.Shape != ShapeList {
		t.Errorf("Shape = %v, want ShapeList", s.Shape)
	}
	if s.Subkey != "jobs" {
		t.Errorf("Subkey = %q, want %q", s.Subkey, "jobs")
	}
	if !s.NoDiscriminator {
		t.Error("NoDiscriminator = false, want true")
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestCronFieldsPresent(t *testing.T) {
	s, _ := Get("cron")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}

	// All four fields must be present
	for _, name := range []string{"name", "schedule", "prompt", "model"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("field %q missing", name)
		}
	}

	// No "provider" field
	if _, ok := byName["provider"]; ok {
		t.Error("unexpected 'provider' field found — cron uses NoDiscriminator")
	}

	// Kinds
	if f := byName["name"]; f != nil && f.Kind != FieldString {
		t.Errorf("name.Kind = %s, want string", f.Kind)
	}
	if f := byName["schedule"]; f != nil && f.Kind != FieldString {
		t.Errorf("schedule.Kind = %s, want string", f.Kind)
	}
	if f := byName["prompt"]; f != nil && f.Kind != FieldString {
		t.Errorf("prompt.Kind = %s, want string", f.Kind)
	}
	if f := byName["model"]; f != nil && f.Kind != FieldString {
		t.Errorf("model.Kind = %s, want string", f.Kind)
	}
}

func TestCronRequiredFields(t *testing.T) {
	s, _ := Get("cron")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range []string{"name", "schedule", "prompt"} {
		f, ok := byName[name]
		if !ok {
			continue
		}
		if !f.Required {
			t.Errorf("field %q: Required = false, want true", name)
		}
	}
	// model is optional
	if f := byName["model"]; f != nil && f.Required {
		t.Error("model.Required = true, want false (optional override)")
	}
}

func TestCronModelDatalistSource(t *testing.T) {
	s, _ := Get("cron")
	var model *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "model" {
			model = &s.Fields[i]
			break
		}
	}
	if model == nil {
		t.Fatal("model field missing")
	}
	if model.DatalistSource == nil {
		t.Fatal("model.DatalistSource is nil, want {Section: \"providers\", Field: \"model\"}")
	}
	if model.DatalistSource.Section != "providers" {
		t.Errorf("DatalistSource.Section = %q, want %q", model.DatalistSource.Section, "providers")
	}
	if model.DatalistSource.Field != "model" {
		t.Errorf("DatalistSource.Field = %q, want %q", model.DatalistSource.Field, "model")
	}
}
