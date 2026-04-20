package descriptor

import (
	"strings"
	"testing"
)

func TestFallbackProvidersSectionRegistered(t *testing.T) {
	s, ok := Get("fallback_providers")
	if !ok {
		t.Fatal(`Get("fallback_providers") returned ok=false — did fallback_providers.go init() register?`)
	}
	if s.GroupID != "models" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "models")
	}
	if s.Shape != ShapeList {
		t.Errorf("Shape = %v, want ShapeList", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
	if s.Summary == "" {
		t.Error("Summary is empty")
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

func TestFallbackProvidersMirrorsProvidersSchema(t *testing.T) {
	// Sanity: same field schema as the primary providers section. If providers
	// adds a field (e.g. "organization"), fallback_providers should follow —
	// they describe the same struct.
	prim, ok := Get("providers")
	if !ok {
		t.Skip("providers not registered — 4b regression")
	}
	fb, ok := Get("fallback_providers")
	if !ok {
		t.Fatal("fallback_providers not registered")
	}
	if len(prim.Fields) != len(fb.Fields) {
		t.Fatalf("field count diverged: primary=%d fallback=%d", len(prim.Fields), len(fb.Fields))
	}
	for i := range prim.Fields {
		if prim.Fields[i].Name != fb.Fields[i].Name {
			t.Errorf("[%d] name divergence: primary=%q fallback=%q",
				i, prim.Fields[i].Name, fb.Fields[i].Name)
		}
		if prim.Fields[i].Kind != fb.Fields[i].Kind {
			t.Errorf("[%d] kind divergence: primary=%v fallback=%v",
				i, prim.Fields[i].Kind, fb.Fields[i].Kind)
		}
	}
}

func TestFallbackProvidersProviderEnumPopulatedFromFactory(t *testing.T) {
	s, _ := Get("fallback_providers")
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
		t.Fatal("provider.Enum empty — did fallback_providers.go import provider/factory?")
	}
	for _, got := range provider.Enum {
		if strings.TrimSpace(got) != got || got == "" {
			t.Errorf("provider enum entry %q has whitespace or is blank", got)
		}
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
