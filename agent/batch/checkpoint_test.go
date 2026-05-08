package batch

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestCheckpoint_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.jsonl")

	cp, err := OpenCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cp.MarkDone("q1"); err != nil {
		t.Fatal(err)
	}
	if err := cp.MarkDone("q2"); err != nil {
		t.Fatal(err)
	}
	_ = cp.Close()

	seen, err := LoadCheckpointSet(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("len = %d", len(seen))
	}
	if !seen["q1"] || !seen["q2"] {
		t.Errorf("missing id: %+v", seen)
	}
}

func TestLoadCheckpointSet_MissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	seen, err := LoadCheckpointSet(filepath.Join(dir, "none.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 0 {
		t.Errorf("expected empty, got %v", seen)
	}
}

func TestCheckpoint_ConcurrentMarkDone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.jsonl")

	cp, err := OpenCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = cp.MarkDone(string(rune('A' + (i % 26))))
		}(i)
	}
	wg.Wait()
	_ = cp.Close()

	seen, err := LoadCheckpointSet(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) < 1 {
		t.Errorf("expected at least 1 entry, got %d", len(seen))
	}
}

func TestCheckpoint_ResumesAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.jsonl")

	cp, err := OpenCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = cp.MarkDone("a")
	_ = cp.Close()

	cp2, err := OpenCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = cp2.MarkDone("b")
	_ = cp2.Close()

	seen, _ := LoadCheckpointSet(path)
	if !seen["a"] || !seen["b"] {
		t.Errorf("expected a+b, got %v", seen)
	}
}
