package skills

import pskills "github.com/odysseythink/pantheon/extensions/skills"

// Loader is a re-export of the pantheon loader.
type Loader = pskills.Loader

// NewLoader constructs a loader rooted at home.
func NewLoader(home string) *Loader { return pskills.NewLoader(home) }
