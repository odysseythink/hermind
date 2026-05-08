package descriptor

import "testing"

func TestPresenceSectionRegistered(t *testing.T) {
	s, ok := Get("presence")
	if !ok {
		t.Fatalf(`Get("presence") returned ok=false — did presence.go init() register?`)
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

func TestPresenceSectionFields(t *testing.T) {
	s, _ := Get("presence")
	want := []struct {
		name        string
		kind        FieldKind
		def         any
		gated       bool // expects visible_when on sleep_window.enabled=true
	}{
		{"http_idle_absent_after_seconds", FieldInt, 300, false},
		{"sleep_window.enabled", FieldBool, false, false},
		{"sleep_window.start", FieldString, "22:00", true},
		{"sleep_window.end", FieldString, "06:00", true},
		{"sleep_window.timezone", FieldString, "", true},
	}
	if len(s.Fields) != len(want) {
		t.Fatalf("field count = %d, want %d: %+v", len(s.Fields), len(want), s.Fields)
	}
	for i, w := range want {
		f := s.Fields[i]
		if f.Name != w.name {
			t.Errorf("field[%d].Name = %q, want %q", i, f.Name, w.name)
		}
		if f.Kind != w.kind {
			t.Errorf("field[%d=%s].Kind = %s, want %s", i, f.Name, f.Kind, w.kind)
		}
		if f.Default != w.def {
			t.Errorf("field[%d=%s].Default = %v (%T), want %v (%T)", i, f.Name, f.Default, f.Default, w.def, w.def)
		}
		if w.gated {
			if f.VisibleWhen == nil {
				t.Errorf("field[%d=%s].VisibleWhen is nil; want gate on sleep_window.enabled=true", i, f.Name)
				continue
			}
			if f.VisibleWhen.Field != "sleep_window.enabled" {
				t.Errorf("field[%d=%s].VisibleWhen.Field = %q, want %q", i, f.Name, f.VisibleWhen.Field, "sleep_window.enabled")
			}
			if f.VisibleWhen.Equals != true {
				t.Errorf("field[%d=%s].VisibleWhen.Equals = %v, want true", i, f.Name, f.VisibleWhen.Equals)
			}
		} else if f.VisibleWhen != nil {
			t.Errorf("field[%d=%s].VisibleWhen = %+v, want nil", i, f.Name, f.VisibleWhen)
		}
	}
}
