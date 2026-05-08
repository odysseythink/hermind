package descriptor

import "testing"

func TestAuxiliarySectionRegistered(t *testing.T) {
	s, ok := Get("auxiliary")
	if !ok {
		t.Fatal(`Get("auxiliary") returned ok=false`)
	}
	if s.GroupID != "runtime" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "runtime")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap (auxiliary is a regular map section)", s.Shape)
	}

	want := map[string]FieldKind{
		"provider": FieldEnum,
		"base_url": FieldString,
		"api_key":  FieldSecret,
		"model":    FieldString,
	}
	got := map[string]FieldKind{}
	for _, f := range s.Fields {
		got[f.Name] = f.Kind
	}
	for name, kind := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if g != kind {
			t.Errorf("field %q: Kind = %s, want %s", name, g, kind)
		}
	}

	// Sanity — no field is Required; "all blank = reuse main provider" is valid.
	for _, f := range s.Fields {
		if f.Required {
			t.Errorf("field %q: Required = true, want false (blank auxiliary reuses main provider)", f.Name)
		}
	}
}

func TestAuxiliaryProviderEnumPopulatedFromFactory(t *testing.T) {
	s, _ := Get("auxiliary")
	var provider *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "provider" {
			provider = &s.Fields[i]
			break
		}
	}
	if provider == nil {
		t.Fatal("provider field not found")
	}
	if len(provider.Enum) == 0 {
		t.Fatal("provider.Enum is empty — did auxiliary.go import provider/factory correctly?")
	}
	// Sanity floor — the enum must include at least the two reference providers.
	has := map[string]bool{}
	for _, v := range provider.Enum {
		has[v] = true
	}
	for _, want := range []string{"anthropic", "openai"} {
		if !has[want] {
			t.Errorf("provider.Enum missing %q; got %v", want, provider.Enum)
		}
	}
}
