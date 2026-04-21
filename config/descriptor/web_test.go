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
