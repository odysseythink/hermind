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
		{FieldMultiSelect, "multiselect"},
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
		if s.Shape == ShapeScalar && len(s.Fields) != 1 {
			t.Errorf("section %q: ShapeScalar requires exactly 1 field, got %d",
				s.Key, len(s.Fields))
		}
		if s.Shape == ShapeKeyedMap {
			if len(s.Fields) == 0 {
				t.Errorf("section %q: ShapeKeyedMap requires at least 1 field", s.Key)
			}
			if !s.NoDiscriminator {
				var providerEnums int
				for _, f := range s.Fields {
					if f.Name == "provider" && f.Kind == FieldEnum {
						providerEnums++
					}
				}
				if providerEnums != 1 {
					t.Errorf("section %q: ShapeKeyedMap requires exactly one FieldEnum named \"provider\" (got %d)",
						s.Key, providerEnums)
				}
			}
		}
		if s.Shape == ShapeList {
			if len(s.Fields) == 0 {
				t.Errorf("section %q: ShapeList requires at least 1 field", s.Key)
			}
			if !s.NoDiscriminator {
				var providerEnums int
				for _, f := range s.Fields {
					if f.Name == "provider" && f.Kind == FieldEnum {
						providerEnums++
					}
				}
				if providerEnums != 1 {
					t.Errorf("section %q: ShapeList requires exactly one FieldEnum named \"provider\" (got %d)",
						s.Key, providerEnums)
				}
			}
		}
	}
}

func TestSectionShape_Constants(t *testing.T) {
	// ShapeMap must be the zero value so existing sections (storage, agent,
	// terminal, logging, metrics, tracing) declared without a Shape field
	// stay map-shaped without touching their Register calls.
	if ShapeMap != 0 {
		t.Errorf("ShapeMap = %d, want 0 (zero value)", ShapeMap)
	}
	if ShapeScalar == ShapeMap {
		t.Error("ShapeScalar equals ShapeMap — they must be distinct")
	}
}

func TestShapeScalarInvariant_FlagsBadFieldCount(t *testing.T) {
	// Seed a deliberately broken ShapeScalar section (2 fields instead of 1)
	// and verify the invariant logic inside TestSectionInvariants would flag
	// it. Runs the invariant check inline because TestSectionInvariants walks
	// the registry as-it-was-at-startup, which doesn't include this seed.
	key := "__test_shape_scalar_bad"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeScalar,
		Fields: []FieldSpec{
			{Name: "a", Label: "A", Kind: FieldString},
			{Name: "b", Label: "B", Kind: FieldString},
		},
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key {
			continue
		}
		if s.Shape == ShapeScalar && len(s.Fields) != 1 {
			fired = true
		}
	}
	if !fired {
		t.Error("invariant did not flag a ShapeScalar section with 2 fields — infrastructure bug")
	}
}

func TestSectionShape_KeyedMapConstantDistinct(t *testing.T) {
	// ShapeKeyedMap must be distinct from both ShapeMap (zero-value default)
	// and ShapeScalar (added in Stage 4a). A collision would silently break
	// the schema DTO's shape-string emission.
	if ShapeKeyedMap == ShapeMap {
		t.Error("ShapeKeyedMap equals ShapeMap — they must be distinct")
	}
	if ShapeKeyedMap == ShapeScalar {
		t.Error("ShapeKeyedMap equals ShapeScalar — they must be distinct")
	}
}

func TestShapeKeyedMapInvariant_FlagsMissingProviderEnum(t *testing.T) {
	// Seed a ShapeKeyedMap section without the required provider-enum field
	// and verify the invariant logic would reject it.
	key := "__test_keyed_map_no_provider"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeKeyedMap,
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Base URL", Kind: FieldString},
		},
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key {
			continue
		}
		if s.Shape != ShapeKeyedMap {
			continue
		}
		var providerEnums int
		for _, f := range s.Fields {
			if f.Name == "provider" && f.Kind == FieldEnum {
				providerEnums++
			}
		}
		if providerEnums != 1 {
			fired = true
		}
	}
	if !fired {
		t.Error("invariant did not flag ShapeKeyedMap without a provider-enum field")
	}
}

func TestShapeKeyedMapInvariant_FlagsEmptyFields(t *testing.T) {
	key := "__test_keyed_map_empty_fields"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeKeyedMap,
		Fields:  nil,
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key {
			continue
		}
		if s.Shape == ShapeKeyedMap && len(s.Fields) == 0 {
			fired = true
		}
	}
	if !fired {
		t.Error("invariant did not flag ShapeKeyedMap with empty Fields")
	}
}

func TestSectionShape_ListConstantDistinct(t *testing.T) {
	// ShapeList must be distinct from ShapeMap (zero value), ShapeScalar
	// (Stage 4a), and ShapeKeyedMap (Stage 4b). A collision would silently
	// break the schema DTO's shape-string emission.
	if ShapeList == ShapeMap {
		t.Error("ShapeList equals ShapeMap — they must be distinct")
	}
	if ShapeList == ShapeScalar {
		t.Error("ShapeList equals ShapeScalar — they must be distinct")
	}
	if ShapeList == ShapeKeyedMap {
		t.Error("ShapeList equals ShapeKeyedMap — they must be distinct")
	}
}

func TestShapeListInvariant_FlagsMissingProviderEnum(t *testing.T) {
	// Seed a ShapeList section without a provider-type discriminator and
	// verify the invariant logic would reject it. fallback_providers
	// mandates a provider enum the same way providers (4b) does.
	key := "__test_list_no_provider"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeList,
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Base URL", Kind: FieldString},
		},
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key {
			continue
		}
		if s.Shape != ShapeList {
			continue
		}
		var providerEnums int
		for _, f := range s.Fields {
			if f.Name == "provider" && f.Kind == FieldEnum {
				providerEnums++
			}
		}
		if providerEnums != 1 {
			fired = true
		}
	}
	if !fired {
		t.Error("invariant did not flag ShapeList without a provider-enum field")
	}
}

func TestFieldSpec_DatalistSourceField(t *testing.T) {
	// A FieldSpec may carry an optional DatalistSource pointer. Nil by default;
	// when set, the DTO emission surfaces it and the UI renders a datalist.
	key := "__test_datalist_source"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeScalar,
		Fields: []FieldSpec{
			{
				Name:  "thing",
				Label: "Thing",
				Kind:  FieldString,
				DatalistSource: &DatalistSource{
					Section: "providers",
					Field:   "model",
				},
			},
		},
	})
	s, _ := Get(key)
	if len(s.Fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(s.Fields))
	}
	ds := s.Fields[0].DatalistSource
	if ds == nil {
		t.Fatal("DatalistSource is nil")
	}
	if ds.Section != "providers" || ds.Field != "model" {
		t.Errorf("DatalistSource = %+v, want {Section: providers, Field: model}", ds)
	}
}

func TestFieldSpec_DatalistSourceDefaultsToNil(t *testing.T) {
	var f FieldSpec
	if f.DatalistSource != nil {
		t.Errorf("zero-value DatalistSource = %+v, want nil", f.DatalistSource)
	}
}

func TestShapeListInvariant_FlagsEmptyFields(t *testing.T) {
	key := "__test_list_empty_fields"
	defer delete(registry, key)
	Register(Section{
		Key:     key,
		Label:   "Test",
		GroupID: "runtime",
		Shape:   ShapeList,
		Fields:  nil,
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key {
			continue
		}
		if s.Shape == ShapeList && len(s.Fields) == 0 {
			fired = true
		}
	}
	if !fired {
		t.Error("invariant did not flag ShapeList with empty Fields")
	}
}

func TestSectionSubkeyDefaultsToEmpty(t *testing.T) {
	key := "__test_subkey_default"
	defer delete(registry, key)
	Register(Section{
		Key: key, Label: "Test", GroupID: "runtime",
		Shape:  ShapeMap,
		Fields: []FieldSpec{{Name: "f", Label: "F", Kind: FieldString}},
	})
	s, _ := Get(key)
	if s.Subkey != "" {
		t.Errorf("Subkey default = %q, want empty", s.Subkey)
	}
	if s.NoDiscriminator {
		t.Error("NoDiscriminator default = true, want false")
	}
}

func TestShapeKeyedMapInvariant_NoDiscriminatorSkipsProviderRequirement(t *testing.T) {
	// Mirror the style of TestShapeKeyedMapInvariant_FlagsMissingProviderEnum
	// but with NoDiscriminator: true — the invariant must NOT fire.
	key := "__test_keyed_map_no_discriminator"
	defer delete(registry, key)
	Register(Section{
		Key: key, Label: "Test", GroupID: "runtime",
		Shape:           ShapeKeyedMap,
		Subkey:          "servers",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "command", Label: "Command", Kind: FieldString, Required: true},
		},
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key || s.Shape != ShapeKeyedMap {
			continue
		}
		if s.NoDiscriminator {
			continue
		}
		var providerEnums int
		for _, f := range s.Fields {
			if f.Name == "provider" && f.Kind == FieldEnum {
				providerEnums++
			}
		}
		if providerEnums != 1 {
			fired = true
		}
	}
	if fired {
		t.Error("invariant fired on NoDiscriminator ShapeKeyedMap — should have been skipped")
	}
}

func TestShapeListInvariant_NoDiscriminatorSkipsProviderRequirement(t *testing.T) {
	key := "__test_list_no_discriminator"
	defer delete(registry, key)
	Register(Section{
		Key: key, Label: "Test", GroupID: "runtime",
		Shape:           ShapeList,
		Subkey:          "jobs",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "name", Label: "Name", Kind: FieldString, Required: true},
		},
	})

	var fired bool
	for _, s := range All() {
		if s.Key != key || s.Shape != ShapeList {
			continue
		}
		if s.NoDiscriminator {
			continue
		}
		var providerEnums int
		for _, f := range s.Fields {
			if f.Name == "provider" && f.Kind == FieldEnum {
				providerEnums++
			}
		}
		if providerEnums != 1 {
			fired = true
		}
	}
	if fired {
		t.Error("invariant fired on NoDiscriminator ShapeList — should have been skipped")
	}
}
