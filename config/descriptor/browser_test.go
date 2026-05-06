package descriptor

import "testing"

func TestBrowserSectionRegistered(t *testing.T) {
	s, ok := Get("browser")
	if !ok {
		t.Fatal("Get(\"browser\") returned ok=false — did browser.go init() register?")
	}
	if s.GroupID != "advanced" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "advanced")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestBrowserProviderEnum(t *testing.T) {
	s, _ := Get("browser")
	var p *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "provider" {
			p = &s.Fields[i]
			break
		}
	}
	if p == nil {
		t.Fatal("provider field missing")
	}
	if p.Kind != FieldEnum {
		t.Errorf("provider.Kind = %s, want enum", p.Kind)
	}
	want := map[string]bool{"browserbase": true, "camofox": true}
	got := map[string]bool{}
	for _, v := range p.Enum {
		got[v] = true
	}
	for v := range want {
		if !got[v] {
			t.Errorf("provider.Enum missing %q, got %v", v, p.Enum)
		}
	}
}

func TestBrowserbaseApiKeyIsSecretGatedByProvider(t *testing.T) {
	s, _ := Get("browser")
	var f *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "browserbase.api_key" {
			f = &s.Fields[i]
			break
		}
	}
	if f == nil {
		t.Fatal("browserbase.api_key missing")
	}
	if f.Kind != FieldSecret {
		t.Errorf("Kind = %s, want secret", f.Kind)
	}
	if f.VisibleWhen == nil ||
		f.VisibleWhen.Field != "provider" ||
		f.VisibleWhen.Equals != "browserbase" {
		t.Errorf("VisibleWhen = %+v, want {provider=browserbase}", f.VisibleWhen)
	}
}

func TestCamofoxFieldsGatedByProvider(t *testing.T) {
	s, _ := Get("browser")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range []string{"camofox.base_url", "camofox.managed_persistence"} {
		f, ok := byName[name]
		if !ok {
			t.Errorf("field %q missing", name)
			continue
		}
		if f.VisibleWhen == nil ||
			f.VisibleWhen.Field != "provider" ||
			f.VisibleWhen.Equals != "camofox" {
			t.Errorf("field %q: VisibleWhen = %+v, want {provider=camofox}", name, f.VisibleWhen)
		}
	}
}
