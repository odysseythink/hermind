package trajectory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSink appends episodes as JSONL to a local file. Writes are
// mutex-serialized and each line is fsync'd so a crash at any moment
// never leaves a torn episode on disk — a property the Python trainer
// depends on when tailing the file while hermind is still running.
type FileSink struct {
	mu     sync.Mutex
	f      *os.File
	bw     *bufio.Writer
	closed bool
}

// NewFileSink opens path for append, creating it (and any parent
// directories) if necessary. The returned sink is safe to pass to any
// number of goroutines.
func NewFileSink(path string) (*FileSink, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("trajectory: mkdir: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("trajectory: open sink: %w", err)
	}
	return &FileSink{f: f, bw: bufio.NewWriter(f)}, nil
}

// Write serializes ep as a single JSON line, flushes the buffer, and
// fsyncs the underlying file. The context is accepted for interface
// symmetry but is not plumbed into local disk I/O.
func (s *FileSink) Write(_ context.Context, ep Episode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("trajectory: file sink closed")
	}
	if err := ToJSONL(s.bw, ep); err != nil {
		return err
	}
	if err := s.bw.Flush(); err != nil {
		return fmt.Errorf("trajectory: flush: %w", err)
	}
	if err := s.f.Sync(); err != nil {
		return fmt.Errorf("trajectory: fsync: %w", err)
	}
	return nil
}

// Close flushes and closes the underlying file. It is safe to call
// multiple times — subsequent calls are no-ops.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var firstErr error
	if s.bw != nil {
		if err := s.bw.Flush(); err != nil {
			firstErr = err
		}
		s.bw = nil
	}
	if s.f != nil {
		if err := s.f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.f = nil
	}
	return firstErr
}
