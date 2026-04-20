package descriptor

import "testing"

func TestMemorySectionRegistered(t *testing.T) {
	s, ok := Get("memory")
	if !ok {
		t.Fatalf("Get(\"memory\") returned ok=false — did memory.go init() register?")
	}
	if s.GroupID != "memory" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "memory")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestMemoryProviderIsEnumWithAllBackends(t *testing.T) {
	s, _ := Get("memory")
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
	if provider.Kind != FieldEnum {
		t.Errorf("provider.Kind = %s, want enum", provider.Kind)
	}
	want := map[string]bool{
		"": true, "honcho": true, "mem0": true, "supermemory": true,
		"hindsight": true, "retaindb": true, "openviking": true,
		"byterover": true, "holographic": true,
	}
	got := map[string]bool{}
	for _, v := range provider.Enum {
		got[v] = true
	}
	for v := range want {
		if !got[v] {
			t.Errorf("provider.Enum missing %q, got %v", v, provider.Enum)
		}
	}
}

func TestMemoryAPIKeysAreSecretAndGatedByProvider(t *testing.T) {
	s, _ := Get("memory")
	wantSecrets := []string{
		"honcho.api_key", "mem0.api_key", "supermemory.api_key",
		"hindsight.api_key", "retaindb.api_key", "openviking.api_key",
	}
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range wantSecrets {
		f, ok := byName[name]
		if !ok {
			t.Errorf("field %q not found", name)
			continue
		}
		if f.Kind != FieldSecret {
			t.Errorf("field %q: Kind = %s, want secret", name, f.Kind)
		}
		if f.VisibleWhen == nil {
			t.Errorf("field %q: VisibleWhen is nil", name)
			continue
		}
		if f.VisibleWhen.Field != "provider" {
			t.Errorf("field %q: VisibleWhen.Field = %q, want \"provider\"",
				name, f.VisibleWhen.Field)
		}
		// Backend name is the first dotted segment — equals check target.
		// e.g. "honcho.api_key" -> want Equals == "honcho".
		var backend string
		for i := 0; i < len(name); i++ {
			if name[i] == '.' {
				backend = name[:i]
				break
			}
		}
		if f.VisibleWhen.Equals != backend {
			t.Errorf("field %q: VisibleWhen.Equals = %v, want %q",
				name, f.VisibleWhen.Equals, backend)
		}
	}
}

func TestMemoryByteroverFieldsAreGated(t *testing.T) {
	s, _ := Get("memory")
	byName := map[string]*FieldSpec{}
	for i := range s.Fields {
		byName[s.Fields[i].Name] = &s.Fields[i]
	}
	for _, name := range []string{"byterover.brv_path", "byterover.cwd"} {
		f, ok := byName[name]
		if !ok {
			t.Errorf("field %q not found", name)
			continue
		}
		if f.Kind != FieldString {
			t.Errorf("field %q: Kind = %s, want string", name, f.Kind)
		}
		if f.VisibleWhen == nil || f.VisibleWhen.Field != "provider" ||
			f.VisibleWhen.Equals != "byterover" {
			t.Errorf("field %q: VisibleWhen = %+v, want {provider=byterover}",
				name, f.VisibleWhen)
		}
	}
}

func TestMemoryHindsightBudgetIsEnum(t *testing.T) {
	s, _ := Get("memory")
	var budget *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "hindsight.budget" {
			budget = &s.Fields[i]
			break
		}
	}
	if budget == nil {
		t.Fatal("hindsight.budget not found")
	}
	if budget.Kind != FieldEnum {
		t.Errorf("hindsight.budget.Kind = %s, want enum", budget.Kind)
	}
	want := map[string]bool{"low": true, "mid": true, "high": true}
	for _, v := range budget.Enum {
		delete(want, v)
	}
	if len(want) > 0 {
		t.Errorf("hindsight.budget.Enum missing %v, got %v", want, budget.Enum)
	}
}
