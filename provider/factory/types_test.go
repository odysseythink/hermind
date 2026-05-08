package factory

import (
	"sort"
	"testing"
)

func TestTypesReturnsSortedPrimaryNames(t *testing.T) {
	got := Types()
	if len(got) == 0 {
		t.Fatal("Types() returned empty slice — expected the registered provider names")
	}
	// Must be sorted — auxiliary descriptor renders the enum in order.
	sortedCopy := append([]string(nil), got...)
	sort.Strings(sortedCopy)
	for i := range got {
		if got[i] != sortedCopy[i] {
			t.Fatalf("Types() not sorted: got %v", got)
		}
	}
	// Sanity floor — the dropdown is useless without at least these two.
	want := map[string]bool{"anthropic": true, "openai": true}
	for _, t := range got {
		delete(want, t)
	}
	if len(want) > 0 {
		t.Errorf("Types() missing expected primaries: %v; got %v", want, got)
	}
}

func TestTypesExcludesAliases(t *testing.T) {
	// Aliases like "glm" / "moonshot" / "ernie" resolve in factory.New
	// but must NOT appear in Types() — the UI dropdown should show one
	// entry per primary provider, not both the primary and its aliases.
	got := map[string]bool{}
	for _, t := range Types() {
		got[t] = true
	}
	for _, alias := range []string{"glm", "moonshot", "ernie"} {
		if got[alias] {
			t.Errorf("Types() includes alias %q; aliases belong in factory.New only", alias)
		}
	}
}
