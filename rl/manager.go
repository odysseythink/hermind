package rl

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
)

type RunStatus struct {
	RunID     string    `json:"run_id"`
	State     string    `json:"state"` // "running", "stopped", "failed", "unknown"
	Command   string    `json:"command"`
	StartTime time.Time `json:"start_time"`
	PID       int       `json:"pid,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type run struct {
	id        string
	cmd       *exec.Cmd
	startTime time.Time
	done      chan struct{}
	err       error
}

type Manager struct {
	mu   sync.Mutex
	runs map[string]*run
}

func NewManager() *Manager {
	return &Manager{runs: make(map[string]*run)}
}

func (m *Manager) Start(ctx context.Context, command string, args []string) (string, error) {
	id := uuid.New().String()[:8]
	cmd := exec.CommandContext(ctx, command, args...)
	applyProcGroup(cmd)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("rl: start %s: %w", command, err)
	}

	r := &run{
		id:        id,
		cmd:       cmd,
		startTime: time.Now().UTC(),
		done:      make(chan struct{}),
	}

	go func() {
		r.err = cmd.Wait()
		close(r.done)
		slog.Info("rl: process exited", "run_id", id, "err", r.err)
	}()

	m.mu.Lock()
	m.runs[id] = r
	m.mu.Unlock()

	slog.Info("rl: started", "run_id", id, "pid", cmd.Process.Pid, "command", command)
	return id, nil
}

func (m *Manager) Status(runID string) RunStatus {
	m.mu.Lock()
	r, ok := m.runs[runID]
	m.mu.Unlock()

	if !ok {
		return RunStatus{RunID: runID, State: "unknown"}
	}

	select {
	case <-r.done:
		state := "stopped"
		errMsg := ""
		if r.err != nil {
			state = "failed"
			errMsg = r.err.Error()
		}
		return RunStatus{
			RunID: runID, State: state, Command: r.cmd.Path,
			StartTime: r.startTime, Error: errMsg,
		}
	default:
		return RunStatus{
			RunID: runID, State: "running", Command: r.cmd.Path,
			StartTime: r.startTime, PID: r.cmd.Process.Pid,
		}
	}
}

func (m *Manager) Stop(runID string) error {
	m.mu.Lock()
	r, ok := m.runs[runID]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("rl: unknown run %q", runID)
	}

	if r.cmd.Process != nil {
		_ = terminateGroup(r.cmd)
	}

	select {
	case <-r.done:
		return nil
	case <-time.After(30 * time.Second):
	}

	if r.cmd.Process != nil {
		_ = killGroup(r.cmd)
	}

	<-r.done
	return nil
}

func (m *Manager) List() []RunStatus {
	m.mu.Lock()
	ids := make([]string, 0, len(m.runs))
	for id := range m.runs {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	out := make([]RunStatus, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.Status(id))
	}
	return out
}
