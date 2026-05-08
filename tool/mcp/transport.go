// tool/mcp/transport.go
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Transport is the interface every MCP wire transport implements.
// Plan 6b provides only StdioTransport; HTTP/SSE is deferred.
type Transport interface {
	// Start opens the transport (e.g. launches the subprocess).
	Start(ctx context.Context) error
	// Send writes one JSON-RPC message frame to the server.
	Send(msg any) error
	// Recv reads one JSON-RPC message frame from the server.
	// Returns io.EOF when the server closes the connection.
	Recv() (json.RawMessage, error)
	// Close terminates the transport and any subprocess it owns.
	Close() error
}

// StdioTransport launches an MCP server as a subprocess and communicates
// via its stdin/stdout using newline-delimited JSON.
//
// Stderr is piped to the parent process's stderr so operators can see
// server logs during development. Plan 6b does not capture stderr.
type StdioTransport struct {
	Command string
	Args    []string
	Env     map[string]string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader *bufio.Reader

	mu     sync.Mutex
	closed bool
}

// NewStdioTransport creates a new StdioTransport from a command, args, and env.
func NewStdioTransport(command string, args []string, env map[string]string) *StdioTransport {
	return &StdioTransport{
		Command: command,
		Args:    args,
		Env:     env,
	}
}

// Start launches the subprocess and wires stdin/stdout.
func (t *StdioTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd != nil {
		return errors.New("mcp stdio: already started")
	}

	cmd := exec.CommandContext(ctx, t.Command, t.Args...)

	// Inherit parent env, then layer our overrides.
	if len(t.Env) > 0 {
		// Start from parent env so PATH and common vars are present.
		cmd.Env = append(cmd.Env, osEnvironSnapshot()...)
		for k, v := range t.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Stderr goes to the parent's stderr — useful for debugging.
	cmd.Stderr = nil // let exec set it to os.Stderr if nil? Actually we want to pipe it.

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp stdio: start %s: %w", t.Command, err)
	}

	t.cmd = cmd
	t.stdin = stdin
	t.stdout = stdout
	t.reader = bufio.NewReaderSize(stdout, 1024*1024) // 1 MB line buffer
	return nil
}

// Send writes a JSON-RPC message to the subprocess's stdin as a single line.
func (t *StdioTransport) Send(msg any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return errors.New("mcp stdio: transport closed")
	}
	if t.stdin == nil {
		return errors.New("mcp stdio: not started")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mcp stdio: marshal: %w", err)
	}
	data = append(data, '\n')

	if _, err := t.stdin.Write(data); err != nil {
		return fmt.Errorf("mcp stdio: write: %w", err)
	}
	return nil
}

// Recv reads one JSON-RPC message frame (one line) from stdout.
// Returns io.EOF when the server closes its stdout.
func (t *StdioTransport) Recv() (json.RawMessage, error) {
	if t.reader == nil {
		return nil, errors.New("mcp stdio: not started")
	}
	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		// Normalize pipe-closed / file-closed errors to io.EOF so callers
		// can use errors.Is(err, io.EOF) uniformly.
		if len(line) == 0 {
			if isClosedPipeError(err) {
				return nil, io.EOF
			}
			return nil, err
		}
		// Partial line — still try to decode
	}
	// Strip trailing newline and CR
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	if len(line) == 0 {
		return nil, io.EOF
	}
	return json.RawMessage(line), nil
}

// Close shuts down the subprocess. Best-effort: closes stdin to signal
// shutdown, waits briefly for the process, then kills if necessary.
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		// Give the process a moment to exit cleanly, then kill
		done := make(chan error, 1)
		go func() { done <- t.cmd.Wait() }()
		select {
		case <-done:
			// Exited cleanly
		case <-timeoutChan():
			_ = t.cmd.Process.Kill()
			<-done
		}
	}
	return nil
}
