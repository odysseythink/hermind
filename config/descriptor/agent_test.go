package descriptor

import "testing"

func TestAgentSectionRegistered(t *testing.T) {
	s, ok := Get("agent")
	if !ok {
		t.Fatal(`Get("agent") returned ok=false`)
	}
	if s.GroupID != "runtime" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "runtime")
	}
	want := map[string]FieldKind{
		"max_turns":       FieldInt,
		"gateway_timeout": FieldInt,
	}
	got := map[string]FieldKind{}
	for _, f := range s.Fields {
		got[f.Name] = f.Kind
	}
	for name, kind := range want {
		k, ok := got[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if k != kind {
			t.Errorf("field %q: Kind = %s, want %s", name, k, kind)
		}
	}
}

func TestAgentDefaultsMatchConfigDefaults(t *testing.T) {
	s, _ := Get("agent")
	var maxTurns, gwTimeout *FieldSpec
	for i := range s.Fields {
		switch s.Fields[i].Name {
		case "max_turns":
			maxTurns = &s.Fields[i]
		case "gateway_timeout":
			gwTimeout = &s.Fields[i]
		}
	}
	if maxTurns == nil || gwTimeout == nil {
		t.Fatalf("max_turns=%v gateway_timeout=%v", maxTurns, gwTimeout)
	}
	// These mirror config.Default() so the editor's placeholder matches
	// hermind's actual runtime default when the YAML omits the key.
	if maxTurns.Default != 90 {
		t.Errorf("max_turns.Default = %v, want 90", maxTurns.Default)
	}
	if gwTimeout.Default != 1800 {
		t.Errorf("gateway_timeout.Default = %v, want 1800", gwTimeout.Default)
	}
}
