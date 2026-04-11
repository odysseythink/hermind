package gateway

import "testing"

func TestDedupSeenAndEviction(t *testing.T) {
	d := NewDedup(3)
	if d.Seen("a") {
		t.Error("first Seen should be false")
	}
	if !d.Seen("a") {
		t.Error("second Seen should be true")
	}
	// Fill to capacity. After this, order is [a, b, c].
	d.Seen("b")
	d.Seen("c")
	// Insert d — should evict a. order is now [b, c, d].
	d.Seen("d")
	// b, c, d are still present.
	if !d.Seen("b") || !d.Seen("c") || !d.Seen("d") {
		t.Error("b/c/d should still be present")
	}
}

func TestDedupDefaultCapacity(t *testing.T) {
	d := NewDedup(0)
	if d.capacity != 1024 {
		t.Errorf("capacity = %d, want 1024", d.capacity)
	}
}
