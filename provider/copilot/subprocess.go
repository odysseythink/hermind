package copilot

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// subprocess owns the Copilot CLI child process and multiplexes
// JSON-RPC requests over its stdin/stdout.
type subprocess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	writeMu sync.Mutex

	nextID int64

	mu      sync.Mutex
	pending map[int64]chan rpcResponse

	// noteBridge receives server-initiated notifications (e.g.
	// session/update). Tests or higher-level streams consume it.
	noteBridge chan notification

	closed chan struct{}
	// closedFlag is set once readLoop exits so Close can avoid
	// double-closing channels.
	closedFlag atomic.Bool
}

type notification struct {
	Method string
	Params []byte
}

// ensureSubprocess lazily spawns the Copilot CLI child process. Safe
// to call multiple times; subsequent calls return the existing
// subprocess.
func (c *Copilot) ensureSubprocess() (*subprocess, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sub != nil {
		return c.sub, nil
	}
	cmd := exec.Command(c.command, c.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("copilot: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("copilot: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("copilot: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("copilot: start %q: %w", c.command, err)
	}

	s := &subprocess{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewReader(stdout),
		stderr:     stderr,
		pending:    map[int64]chan rpcResponse{},
		noteBridge: make(chan notification, 16),
		closed:     make(chan struct{}),
	}
	// Drain stderr to avoid blocking the child when it chatters.
	go func() {
		_, _ = io.Copy(io.Discard, stderr)
	}()
	go s.readLoop()
	c.sub = s
	return s, nil
}

// Close sends SIGKILL (via Process.Kill) and waits.
func (s *subprocess) Close() error {
	_ = s.stdin.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
	return nil
}
