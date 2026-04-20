package descriptor

import (
	"sort"
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	key := "__test_register"
	defer delete(registry, key)

	want := Section{Key: key, Label: "t", GroupID: "runtime", Fields: []FieldSpec{{Name: "x", Label: "X", Kind: FieldString}}}
	Register(want)

	got, ok := Get(key)
	if !ok {
		t.Fatalf("Get(%q) returned ok=false", key)
	}
	if got.Label != want.Label {
		t.Errorf("Label = %q, want %q", got.Label, want.Label)
	}
}

func TestAllReturnsSortedByKey(t *testing.T) {
	keys := []string{"__t_bbb", "__t_aaa", "__t_ccc"}
	for _, k := range keys {
		Register(Section{Key: k, Label: k, GroupID: "runtime", Fields: []FieldSpec{{Name: "x", Label: "X", Kind: FieldString}}})
	}
	defer func() {
		for _, k := range keys {
			delete(registry, k)
		}
	}()

	all := All()
	var got []string
	for _, s := range all {
		if len(s.Key) > 4 && s.Key[:4] == "__t_" {
			got = append(got, s.Key)
		}
	}
	want := append([]string(nil), keys...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("got %d matching keys, want %d: %v vs %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("All() order = %v, want %v", got, want)
		}
	}
}

func TestFieldKindString(t *testing.T) {
	cases := []struct {
		k    FieldKind
		want string
	}{
		{FieldString, "string"},
		{FieldInt, "int"},
		{FieldBool, "bool"},
		{FieldSecret, "secret"},
		{FieldEnum, "enum"},
		{FieldFloat, "float"},
		{FieldUnknown, "unknown"},
		{FieldKind(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.k.String(); got != tc.want {
			t.Errorf("(%d).String() = %q, want %q", tc.k, got, tc.want)
		}
	}
}

// TestSectionInvariants walks every registered section and enforces the
// guarantees the frontend and redaction loop depend on. Stage 2 has only
// one registered section (Storage); this test protects every stage-3+
// addition from landing broken.
func TestSectionInvariants(t *testing.T) {
	for _, s := range All() {
		if s.Key == "" {
			t.Errorf("section with empty Key: %+v", s)
		}
		if s.Label == "" {
			t.Errorf("section %q: empty Label", s.Key)
		}
		if s.GroupID == "" {
			t.Errorf("section %q: empty GroupID", s.Key)
		}
		if len(s.Fields) == 0 {
			t.Errorf("section %q: no Fields", s.Key)
		}
		names := map[string]bool{}
		for _, f := range s.Fields {
			if f.Kind == FieldUnknown {
				t.Errorf("section %q field %q: Kind is FieldUnknown", s.Key, f.Name)
			}
			if f.Name == "" {
				t.Errorf("section %q: field with empty Name", s.Key)
			}
			if f.Label == "" {
				t.Errorf("section %q field %q: empty Label", s.Key, f.Name)
			}
			if names[f.Name] {
				t.Errorf("section %q: duplicate field Name %q", s.Key, f.Name)
			}
			names[f.Name] = true
			if f.Kind == FieldEnum && len(f.Enum) == 0 {
				t.Errorf("section %q field %q: FieldEnum with empty Enum", s.Key, f.Name)
			}
		}
		// VisibleWhen.Field must reference a sibling field declared in the
		// same section. Evaluated after the names map is fully built so
		// forward references are legal.
		for _, f := range s.Fields {
			if f.VisibleWhen == nil {
				continue
			}
			if !names[f.VisibleWhen.Field] {
				t.Errorf("section %q field %q: VisibleWhen.Field %q is not a sibling field",
					s.Key, f.Name, f.VisibleWhen.Field)
			}
		}
	}
}
