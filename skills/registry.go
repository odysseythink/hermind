package skills

import pskills "github.com/odysseythink/pantheon/extensions/skills"

// Registry is a re-export of pantheon's skill registry.
type Registry = pskills.Registry

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry { return pskills.NewRegistry() }
