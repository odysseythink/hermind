package descriptor

import "testing"

func TestBenchmarkSectionRegistered(t *testing.T) {
	s, ok := Get("benchmark")
	if !ok {
		t.Fatalf(`Get("benchmark") returned ok=false — did benchmark.go init() register?`)
	}
	if s.GroupID != "advanced" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "advanced")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
}

func TestBenchmarkSectionFields(t *testing.T) {
	s, _ := Get("benchmark")
	want := []struct {
		name string
		kind FieldKind
		def  any
		enum []string
	}{
		// Top-level BenchmarkConfig fields
		{"dataset_size", FieldInt, 50, nil},
		{"seed", FieldInt, 42, nil},
		{"judge_model", FieldString, "", nil},
		{"out_dir", FieldString, ".hermind/benchmark", nil},
		// Nested replay.* fields
		{"replay.default_mode", FieldEnum, "cold", []string{"cold", "contextual"}},
		{"replay.default_history_cap", FieldInt, 20, nil},
		{"replay.default_judge", FieldEnum, "none", []string{"none", "pairwise", "rubric+pairwise"}},
		{"replay.out_dir", FieldString, ".hermind/replay", nil},
		{"replay.judge_model", FieldString, "", nil},
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
			t.Errorf("field[%d=%s].Default = %v, want %v", i, f.Name, f.Default, w.def)
		}
		if len(w.enum) > 0 {
			if len(f.Enum) != len(w.enum) {
				t.Errorf("field[%d=%s].Enum len = %d, want %d", i, f.Name, len(f.Enum), len(w.enum))
				continue
			}
			for j, v := range w.enum {
				if f.Enum[j] != v {
					t.Errorf("field[%d=%s].Enum[%d] = %q, want %q", i, f.Name, j, f.Enum[j], v)
				}
			}
		}
		// Benchmark section has no visible_when on any field — replay
		// defaults are always editable regardless of parent toggles.
		if f.VisibleWhen != nil {
			t.Errorf("field[%d=%s].VisibleWhen = %+v, want nil", i, f.Name, f.VisibleWhen)
		}
	}
}
