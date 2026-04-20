package factory

import "sort"

// Types returns the canonical provider-type names the factory knows about,
// sorted alphabetically. Aliases (glm, moonshot, ernie) are deliberately
// excluded — they resolve in New but duplicate their primaries.
//
// Consumed by config/descriptor/auxiliary.go to populate the provider enum.
func Types() []string {
	out := make([]string, 0, len(primary))
	for k := range primary {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
