package factory

import (
	"sort"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

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

// SetConstructorForTest injects a constructor under the given name. Test-only.
// Calls must be paired with ClearConstructorForTest to avoid cross-test pollution.
func SetConstructorForTest(name string, ctor func(cfg config.ProviderConfig) (provider.Provider, error)) {
	primary[name] = ctor
}

// ClearConstructorForTest removes a test-injected constructor.
func ClearConstructorForTest(name string) {
	delete(primary, name)
}
