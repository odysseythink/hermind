package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nousresearch/hermes-agent/message"
)

// TrajectoryEvent is one line in the trajectory dump.
type TrajectoryEvent struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id"`
	Kind      string    `json:"kind"` // "user", "assistant", "tool_call", "tool_result", "usage"
	Content   string    `json:"content,omitempty"`
	ToolName  string    `json:"tool_name,omitempty"`
	Usage     *message.Usage `json:"usage,omitempty"`
}

// TrajectoryWriter appends JSON-lines events to a file on disk.
// Thread-safe via an internal mutex.
type TrajectoryWriter struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// DefaultTrajectoryDir returns the default directory for trajectory
// dumps, usually ~/.hermes/trajectories.
func DefaultTrajectoryDir() string {
	if v := os.Getenv("HERMES_HOME"); v != "" {
		return filepath.Join(v, "trajectories")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hermes", "trajectories")
}

// NewTrajectoryWriter opens (or creates) a trajectory file for a session.
func NewTrajectoryWriter(dir, sessionID string) (*TrajectoryWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("trajectory: open %s: %w", path, err)
	}
	return &TrajectoryWriter{path: path, f: f}, nil
}

// Write appends an event to the trajectory file.
func (tw *TrajectoryWriter) Write(ev TrajectoryEvent) error {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	buf, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	_, err = tw.f.Write(buf)
	return err
}

// Close flushes and releases the file handle.
func (tw *TrajectoryWriter) Close() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.f == nil {
		return nil
	}
	err := tw.f.Close()
	tw.f = nil
	return err
}
