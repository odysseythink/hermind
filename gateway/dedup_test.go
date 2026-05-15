package gateway

import (
	"testing"

	_ "github.com/odysseythink/hermind/logging"
)

func TestDedup_NewDedup_DefaultCapacity(t *testing.T) {
	d := NewDedup(0)
	if d.capacity != 1024 {
		t.Fatalf("expected default capacity 1024, got %d", d.capacity)
	}
}

func TestDedup_Seen_FirstTimeFalse(t *testing.T) {
	d := NewDedup(10)
	if d.Seen("msg1") {
		t.Error("first Seen should return false")
	}
}

func TestDedup_Seen_SecondTimeTrue(t *testing.T) {
	d := NewDedup(10)
	d.Seen("msg1")
	if !d.Seen("msg1") {
		t.Error("second Seen for same key should return true")
	}
}

func TestDedup_Seen_EvictsOldestWhenFull(t *testing.T) {
	d := NewDedup(3)
	d.Seen("a")
	d.Seen("b")
	d.Seen("c")
	// adding "d" evicts "a"; set is now {b,c,d}
	d.Seen("d")
	// Verify "b" and "c" are still present by checking dedup returns true
	if !d.Seen("b") {
		t.Error("'b' should still be in the set after 'd' was added")
	}
	// Verify "a" was truly evicted: a fresh dedup will accept "a" again
	d2 := NewDedup(3)
	d2.Seen("a")
	d2.Seen("b")
	d2.Seen("c")
	d2.Seen("d") // evicts "a"
	// "a" is gone; inserting it again returns false (not seen)
	if d2.Seen("a") {
		t.Error("'a' should have been evicted")
	}
}

func TestDedup_Seen_IndependentKeys(t *testing.T) {
	d := NewDedup(10)
	if d.Seen("x") || d.Seen("y") || d.Seen("z") {
		t.Error("distinct keys should each return false on first use")
	}
	if !d.Seen("x") || !d.Seen("y") || !d.Seen("z") {
		t.Error("each key should be a duplicate on second use")
	}
}
