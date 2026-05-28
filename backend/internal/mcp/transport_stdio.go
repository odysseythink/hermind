package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/odysseythink/mlog"
)

type stdioTransport struct {
	command   string
	args      []string
	env       []string
	client    *client.Client
	tr        transport.Interface
	cmd       *exec.Cmd
	pid       int
	cmdStr    string
	closeOnce sync.Once
	closed    chan struct{}
	mu        sync.Mutex
}

func newStdioTransport(srv *ServerConfig) (Transport, error) {
	if srv.Command == "" {
		return nil, errors.New("stdio transport: missing command")
	}
	return &stdioTransport{
		command: srv.Command,
		args:    srv.Args,
		env:     buildServerEnv(srv),
		cmdStr:  strings.Join(append([]string{srv.Command}, srv.Args...), " "),
		closed:  make(chan struct{}),
	}, nil
}

func (t *stdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	cmd := exec.Command(t.command, t.args...)
	cmd.Env = t.env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdio transport: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdio transport: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stdio transport: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("stdio transport: start command: %w", err)
	}

	t.cmd = cmd
	t.pid = cmd.Process.Pid

	// Drain stderr to mlog so it doesn't block the child.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			mlog.Info("mcp stderr", mlog.String("cmd", t.cmdStr), mlog.String("line", scanner.Text()))
		}
	}()

	tr := transport.NewIO(stdout, stdin, nil)
	if err := tr.Start(ctx); err != nil {
		t.kill()
		return fmt.Errorf("stdio transport: start transport: %w", err)
	}

	c := client.NewClient(tr)
	if err := c.Start(ctx); err != nil {
		t.kill()
		return fmt.Errorf("stdio transport: start client: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "anythingllm", Version: "1.0.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		t.kill()
		return fmt.Errorf("stdio transport: initialize: %w", err)
	}

	t.client = c
	t.tr = tr
	return nil
}

func (t *stdioTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.closed)
		err = t.gracefulKill()
		if t.tr != nil {
			_ = t.tr.Close()
		}
	})
	return err
}

func (t *stdioTransport) gracefulKill() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}
	_ = t.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _ = t.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = t.cmd.Process.Signal(syscall.SIGKILL)
		<-done
	}
	return nil
}

func (t *stdioTransport) kill() {
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
}

func (t *stdioTransport) Ping(ctx context.Context) bool {
	select {
	case <-t.closed:
		return false
	default:
	}
	if t.client == nil {
		return false
	}
	return t.client.Ping(ctx) == nil
}

func (t *stdioTransport) ListTools(ctx context.Context) ([]ToolSchema, error) {
	if t.client == nil {
		return nil, errors.New("stdio transport: not connected")
	}
	raw, err := t.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]ToolSchema, 0, len(raw.Tools))
	for _, r := range raw.Tools {
		schemaJSON, _ := json.Marshal(r.InputSchema)
		out = append(out, ToolSchema{
			Name:        r.Name,
			Description: r.Description,
			InputSchema: schemaJSON,
		})
	}
	return out, nil
}

func (t *stdioTransport) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if t.client == nil {
		return nil, errors.New("stdio transport: not connected")
	}
	return t.client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
}

func (t *stdioTransport) ProcessInfo() *ProcessInfo {
	if t.pid == 0 {
		return nil
	}
	return &ProcessInfo{PID: t.pid, Cmd: t.cmdStr}
}
