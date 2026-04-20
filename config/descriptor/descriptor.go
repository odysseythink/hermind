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
