// Package platforms hosts the gateway adapter registry and the
// descriptors that advertise each adapter's configurable fields.
//
// Each adapter ships a descriptor_<type>.go file whose init() calls
// Register. cli/gateway.go::buildPlatform looks up descriptors here
// instead of carrying a hand-rolled switch.
package platforms

import (
	"context"
	"sort"

	"github.com/odysseythink/hermind/gateway"
)

// FieldKind enumerates the value shapes a descriptor field can carry.
type FieldKind int

const (
	// FieldUnknown is the zero value; descriptor authors must set Kind
	// explicitly to a meaningful value. The TestDescriptorInvariants
	// guard (Task 2) will reject any field left at FieldUnknown.
	FieldUnknown FieldKind = iota
	FieldString
	FieldInt
	FieldBool
	FieldSecret
	FieldEnum
)

// String returns a lowercase name suitable for JSON ("string",
// "secret", etc.). Stage 2 uses this for the schema endpoint.
func (k FieldKind) String() string {
	switch k {
	case FieldUnknown:
		return "unknown"
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
	}
	return "unknown"
}

// FieldSpec describes one configurable field of an adapter.
type FieldSpec struct {
	Name     string    // key under PlatformConfig.Options
	Label    string    // human-readable label
	Help     string    // optional one-line hint
	Kind     FieldKind
	Required bool
	Default  any       // nil when none
	Enum     []string  // only for FieldEnum
}

// Descriptor is the self-describing metadata for a gateway adapter.
//
// Build constructs a running adapter from its Options map. Test does a
// lightweight handshake for the /api/platforms/test endpoint; it may
// be nil until stage 2 populates it.
type Descriptor struct {
	Type        string       // stable identifier, e.g. "telegram"; matches PlatformConfig.Type
	DisplayName string       // human-readable name shown in the UI
	Summary     string       // one-line description; optional
	Fields      []FieldSpec
	Build       func(opts map[string]string) (gateway.Platform, error)
	Test        func(ctx context.Context, opts map[string]string) error
}

var registry = map[string]Descriptor{}

// Register installs d under d.Type, overwriting any prior entry with
// the same Type. Callers invoke this from init() in descriptor files.
func Register(d Descriptor) {
	registry[d.Type] = d
}

// Get returns the descriptor registered for typ. The second return
// value is false when typ is unknown.
func Get(typ string) (Descriptor, bool) {
	d, ok := registry[typ]
	return d, ok
}

// All returns every registered descriptor sorted by Type. Stage 2's
// /api/platforms/schema endpoint uses this to produce a deterministic
// JSON response.
func All() []Descriptor {
	out := make([]Descriptor, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
