package batch

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Checkpoint is a line-oriented, append-only log of completed item
// IDs. On resume, callers call LoadCheckpointSet to get the set of
// already-finished IDs, then skip them.
//
// MarkDone is safe for concurrent use; writes are serialized by an
// internal mutex.
type Checkpoint struct {
	mu sync.Mutex
	f  *os.File
	bw *bufio.Writer
}

type checkpointEntry struct {
	ID string `json:"id"`
}

// OpenCheckpoint opens (creating if missing) the checkpoint file for
// append. The directory is created as needed.
func OpenCheckpoint(path string) (*Checkpoint, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("batch: mkdir checkpoint: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("batch: open checkpoint: %w", err)
	}
	return &Checkpoint{f: f, bw: bufio.NewWriter(f)}, nil
}

// MarkDone appends an entry for id. The write is flushed + fsync'd
// before returning so a crash does not lose the record.
func (c *Checkpoint) MarkDone(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	line, err := json.Marshal(checkpointEntry{ID: id})
	if err != nil {
		return err
	}
	if _, err := c.bw.Write(line); err != nil {
		return err
	}
	if _, err := c.bw.WriteString("\n"); err != nil {
		return err
	}
	if err := c.bw.Flush(); err != nil {
		return err
	}
	return c.f.Sync()
}

// Close flushes remaining buffers and closes the file.
func (c *Checkpoint) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bw != nil {
		_ = c.bw.Flush()
	}
	if c.f != nil {
		return c.f.Close()
	}
	return nil
}

// LoadCheckpointSet reads the file and returns the set of completed
// IDs. A missing file is not an error — returns an empty map.
func LoadCheckpointSet(path string) (map[string]bool, error) {
	seen := map[string]bool{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return seen, nil
	}
	if err != nil {
		return nil, fmt.Errorf("batch: read checkpoint: %w", err)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		s := strings.TrimSpace(line)
		if s != "" {
			var e checkpointEntry
			if json.Unmarshal([]byte(s), &e) == nil && e.ID != "" {
				seen[e.ID] = true
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("batch: read checkpoint: %w", err)
		}
	}
	return seen, nil
}
