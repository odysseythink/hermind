package descriptor

import "testing"

func TestProxySectionRegistered(t *testing.T) {
	s, ok := Get("proxy")
	if !ok {
		t.Fatalf(`Get("proxy") returned ok=false — did proxy.go init() register?`)
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
	if s.Summary == "" {
		t.Error("Summary is empty")
	}
}

func TestProxySectionFields(t *testing.T) {
	s, _ := Get("proxy")
	if len(s.Fields) != 2 {
		t.Fatalf("field count = %d, want 2: %+v", len(s.Fields), s.Fields)
	}

	// enabled — bool, default false, no visible_when
	enabled := s.Fields[0]
	if enabled.Name != "enabled" {
		t.Errorf("Fields[0].Name = %q, want %q", enabled.Name, "enabled")
	}
	if enabled.Kind != FieldBool {
		t.Errorf("Fields[0].Kind = %s, want bool", enabled.Kind)
	}
	if enabled.Default != false {
		t.Errorf("Fields[0].Default = %v, want false", enabled.Default)
	}
	if enabled.VisibleWhen != nil {
		t.Errorf("Fields[0].VisibleWhen = %+v, want nil (always-visible toggle)", enabled.VisibleWhen)
	}
	if enabled.Help == "" {
		t.Error("Fields[0].Help is empty")
	}

	// keep_alive_seconds — int, default 15, visible_when enabled=true
	kas := s.Fields[1]
	if kas.Name != "keep_alive_seconds" {
		t.Errorf("Fields[1].Name = %q, want %q", kas.Name, "keep_alive_seconds")
	}
	if kas.Kind != FieldInt {
		t.Errorf("Fields[1].Kind = %s, want int", kas.Kind)
	}
	if kas.Default != 15 {
		t.Errorf("Fields[1].Default = %v, want 15", kas.Default)
	}
	if kas.VisibleWhen == nil {
		t.Fatalf("Fields[1].VisibleWhen is nil, want gate on enabled=true")
	}
	if kas.VisibleWhen.Field != "enabled" {
		t.Errorf("Fields[1].VisibleWhen.Field = %q, want %q", kas.VisibleWhen.Field, "enabled")
	}
	if kas.VisibleWhen.Equals != true {
		t.Errorf("Fields[1].VisibleWhen.Equals = %v, want true", kas.VisibleWhen.Equals)
	}
}
