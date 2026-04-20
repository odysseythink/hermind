package trajectory

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFileSink_AppendJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	if err := sink.Write(context.Background(), Episode{EpisodeID: "a", Steps: []Step{{From: "user", Value: "hi"}}}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), Episode{EpisodeID: "b", Steps: []Step{{From: "user", Value: "hi2"}}}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var ep Episode
	if err := json.Unmarshal([]byte(lines[0]), &ep); err != nil {
		t.Fatal(err)
	}
	if ep.EpisodeID != "a" {
		t.Errorf("first id = %q", ep.EpisodeID)
	}
}

func TestFileSink_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episodes.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = sink.Write(context.Background(), Episode{EpisodeID: string(rune('a' + (n % 26)))})
		}(i)
	}
	wg.Wait()

	// Re-read: every line must parse as a full episode (if writes
	// interleaved, some would be garbled).
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	count := 0
	for scan.Scan() {
		var ep Episode
		if err := json.Unmarshal(scan.Bytes(), &ep); err != nil {
			t.Errorf("corrupt line: %s", scan.Text())
		}
		count++
	}
	if count != 20 {
		t.Errorf("expected 20 lines, got %d", count)
	}
}

func TestFileSink_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	// Write after close should fail but not panic.
	if err := sink.Write(context.Background(), Episode{EpisodeID: "x"}); err == nil {
		t.Errorf("expected error writing to closed sink")
	}
}

func TestFileSink_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "sub", "episodes.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	if err := sink.Write(context.Background(), Episode{EpisodeID: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("sink file not created: %v", err)
	}
}
