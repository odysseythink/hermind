package descriptor

import "testing"

func TestProvidersSectionRegistered(t *testing.T) {
	s, ok := Get("providers")
	if !ok {
		t.Fatal(`Get("providers") returned ok=false — did providers.go init() register?`)
	}
	if s.GroupID != "models" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "models")
	}
	if s.Shape != ShapeKeyedMap {
		t.Errorf("Shape = %v, want ShapeKeyedMap", s.Shape)
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

	// Required flags — provider and api_key are required; base_url and model optional.
	for _, f := range s.Fields {
		switch f.Name {
		case "provider", "api_key":
			if !f.Required {
				t.Errorf("field %q: Required = false, want true", f.Name)
			}
		case "base_url", "model":
			if f.Required {
				t.Errorf("field %q: Required = true, want false", f.Name)
			}
		}
	}
}

func TestProvidersProviderEnumPopulatedFromFactory(t *testing.T) {
	s, _ := Get("providers")
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
		t.Fatal("provider.Enum is empty — did providers.go import provider/factory correctly?")
	}
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
