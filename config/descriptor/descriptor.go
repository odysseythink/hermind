// Package descriptor hosts the schema descriptors used by /api/config/schema
// and the generic React section editor in web/src/components/ConfigSection.tsx.
//
// Each non-platform config section ships a storage.go / agent.go / …
// file whose init() calls Register. The REST handler exposes All()
// so the frontend can render every section without hand-coding its fields.
package descriptor

import "sort"

// FieldKind enumerates the value shapes a descriptor field can carry.
type FieldKind int

const (
	// FieldUnknown is the zero value; descriptor authors must set Kind
	// explicitly. The invariants test rejects any field left at FieldUnknown.
	FieldUnknown FieldKind = iota
	FieldString
	FieldInt
	FieldBool
	FieldSecret
	FieldEnum
	FieldFloat
)

// String returns a lowercase name suitable for JSON ("string", "secret", …).
func (k FieldKind) String() string {
	switch k {
	case FieldString:
		return "string"
	case FieldInt:
		return "int"
	case FieldBool:
		return "bool"
	case FieldSecret:
		return "secret"
	case FieldEnum:
		return "enum"
	case FieldFloat:
		return "float"
	}
	return "unknown"
}

// SectionShape discriminates how a section's value is laid out in YAML.
//   - ShapeMap      — value is map[string]any (the default; every Stage 2/3 section).
//   - ShapeScalar   — value is a scalar (string/int/bool); the section's Fields
//                     slice must contain exactly 1 entry that carries the Kind,
//                     Label, Help, Required, and Default for the scalar.
//   - ShapeKeyedMap — value is map[string]map[string]any (uniform struct per
//                     instance, e.g. providers). Fields describe the instance
//                     value; exactly one FieldEnum named "provider" must serve
//                     as the type discriminator consumed by the UI's
//                     new-instance dialog and by any per-instance redaction.
//   - ShapeList     — value is []map[string]any (ordered list of uniform
//                     struct elements, e.g. fallback_providers). Fields
//                     describe one element; exactly one FieldEnum named
//                     "provider" must serve as the type discriminator.
//                     Preservation of secret fields is strictly by index —
//                     reorder or delete invalidates the secret-carry for the
//                     affected rows.
type SectionShape int

const (
	ShapeMap SectionShape = iota
	ShapeScalar
	ShapeKeyedMap
	ShapeList
)

// Predicate expresses "show this field only when <Field> equals <Equals>".
// Stage 2 supports exactly one equality check; boolean algebra is YAGNI.
type Predicate struct {
	Field  string
	Equals any
}

// FieldSpec describes one configurable field of a Section.
type FieldSpec struct {
	Name        string     // yaml key: "sqlite_path"
	Label       string     // human-readable label
	Help        string     // optional one-line hint
	Kind        FieldKind
	Required    bool
	Default     any        // nil when none
	Enum        []string   // only for FieldEnum
	VisibleWhen *Predicate // nil = always visible
}

// Section is the schema for one top-level config.Config field
// (e.g. config.Storage at yaml key "storage").
type Section struct {
	Key     string      // "storage" — matches the yaml tag on config.Config
	Label   string
	Summary string
	GroupID string      // "runtime" — which shell group hosts this section
	Shape   SectionShape
	Fields  []FieldSpec
}

var registry = map[string]Section{}

// Register installs s under s.Key, overwriting any prior entry.
// Callers invoke this from init() in a per-section file.
func Register(s Section) {
	registry[s.Key] = s
}

// Get returns the section registered at key. The second return value
// is false when key is unknown.
func Get(key string) (Section, bool) {
	s, ok := registry[key]
	return s, ok
}

// All returns every registered section sorted by Key so the JSON
// response is deterministic.
func All() []Section {
	out := make([]Section, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
