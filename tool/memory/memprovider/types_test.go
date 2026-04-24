package memprovider

import "testing"

func TestInjectedMemoryFields(t *testing.T) {
	m := InjectedMemory{ID: "mc_1", Content: "hello"}
	if m.ID != "mc_1" || m.Content != "hello" {
		t.Fatalf("unexpected fields: %+v", m)
	}
}
