package descriptor

import "testing"

func TestWebSectionRegistered(t *testing.T) {
	s, ok := Get("web")
	if !ok {
		t.Fatal("Get(\"web\") returned ok=false — did web.go init() register?")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestWebSearchProviderEnum(t *testing.T) {
	s, _ := Get("web")
	var p *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "search.provider" {
			p = &s.Fields[i]
			break
		}
	}
	if p == nil {
		t.Fatal("search.provider field missing")
	}
	if p.Kind != FieldEnum {
		t.Errorf("search.provider.Kind = %s, want enum", p.Kind)
	}
	want := map[string]bool{"": true, "tavily": true, "brave": true, "exa": true, "ddg": true}
	got := map[string]bool{}
	for _, v := range p.Enum {
		got[v] = true
	}
	for v := range want {
		if !got[v] {
			t.Errorf("search.provider.Enum missing %q, got %v", v, p.Enum)
		}
	}
}

func TestWebProviderAPIKeysAreGatedByProvider(t *testing.T) {
	s, _ := Get("web")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	cases := []struct {
		field, provider string
	}{
		{"search.providers.tavily.api_key", "tavily"},
		{"search.providers.brave.api_key", "brave"},
		{"search.providers.exa.api_key", "exa"},
	}
	for _, c := range cases {
		f, ok := byName[c.field]
		if !ok {
			t.Errorf("field %q missing", c.field)
			continue
		}
		if f.VisibleWhen == nil {
			t.Errorf("%s: VisibleWhen is nil, want predicate gating on search.provider", c.field)
			continue
		}
		if f.VisibleWhen.Field != "search.provider" {
			t.Errorf("%s: VisibleWhen.Field = %q, want %q", c.field, f.VisibleWhen.Field, "search.provider")
		}
		// Each api_key must be reachable both when its provider is selected
		// AND when "" is selected (auto-select). We accept In or Equals.
		matches := func(value string) bool {
			if f.VisibleWhen.In != nil {
				for _, v := range f.VisibleWhen.In {
					if vs, ok := v.(string); ok && vs == value {
						return true
					}
				}
				return false
			}
			if vs, ok := f.VisibleWhen.Equals.(string); ok {
				return vs == value
			}
			return false
		}
		if !matches(c.provider) {
			t.Errorf("%s: not visible when search.provider=%q", c.field, c.provider)
		}
		if !matches("") {
			t.Errorf("%s: not visible when search.provider=\"\" (auto-select); user can never pre-populate", c.field)
		}
		// Other providers must NOT reveal this key.
		for _, other := range []string{"tavily", "brave", "exa", "ddg"} {
			if other == c.provider {
				continue
			}
			if matches(other) {
				t.Errorf("%s: leaks visible when search.provider=%q", c.field, other)
			}
		}
	}
}

func TestWebProviderAPIKeysAreSecrets(t *testing.T) {
	s, _ := Get("web")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range []string{
		"search.providers.tavily.api_key",
		"search.providers.brave.api_key",
		"search.providers.exa.api_key",
	} {
		f, ok := byName[name]
		if !ok {
			t.Errorf("field %q missing", name)
			continue
		}
		if f.Kind != FieldSecret {
			t.Errorf("%s.Kind = %s, want secret", name, f.Kind)
		}
	}
}
