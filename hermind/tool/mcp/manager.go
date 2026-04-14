// tool/mcp/manager.go
package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/odysseythink/hermind/tool"
)

// ServerConfig is the Manager-facing description of a server.
// It mirrors config.MCPServerConfig but lives in this package to avoid
// a circular import between tool/mcp and config.
type ServerConfig struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// Manager owns a set of running MCP clients and their bridges.
// It is constructed at CLI startup, started once, and closed on exit.
type Manager struct {
	clientVersion string
	registry      *tool.Registry

	mu      sync.Mutex
	clients map[string]*Client
	bridges map[string]*Bridge
}

// NewManager returns an empty Manager bound to the given tool registry.
func NewManager(clientVersion string, registry *tool.Registry) *Manager {
	return &Manager{
		clientVersion: clientVersion,
		registry:      registry,
		clients:       map[string]*Client{},
		bridges:       map[string]*Bridge{},
	}
}

// Start launches every server in cfgs, initializes each client, and registers
// their tools in the shared registry. Errors starting any single server are
// logged-and-continued (returned as a composite error after all attempts).
func (m *Manager) Start(ctx context.Context, cfgs []ServerConfig) error {
	var errs []error
	for _, cfg := range cfgs {
		if err := m.startOne(ctx, cfg); err != nil {
			errs = append(errs, fmt.Errorf("start %s: %w", cfg.Name, err))
		}
	}
	if len(errs) > 0 {
		return joinErrors(errs)
	}
	return nil
}

func (m *Manager) startOne(ctx context.Context, cfg ServerConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("server %s: command is required", cfg.Name)
	}

	transport := NewStdioTransport(cfg.Command, cfg.Args, cfg.Env)
	client := NewClient(transport)

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("transport start: %w", err)
	}

	if _, err := client.Initialize(ctx, "hermes-agent", m.clientVersion); err != nil {
		_ = client.Close()
		return fmt.Errorf("initialize: %w", err)
	}

	bridge := NewBridge(cfg.Name, client, m.registry)
	if _, err := bridge.Register(ctx); err != nil {
		_ = client.Close()
		return fmt.Errorf("register tools: %w", err)
	}

	m.mu.Lock()
	m.clients[cfg.Name] = client
	m.bridges[cfg.Name] = bridge
	m.mu.Unlock()
	return nil
}

// Close stops every running server. Best-effort: errors are logged-and-continued.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, c := range m.clients {
		_ = c.Close()
		delete(m.clients, name)
	}
	return nil
}

// Servers returns the list of running server names (for diagnostics).
func (m *Manager) Servers() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.clients))
	for name := range m.clients {
		out = append(out, name)
	}
	return out
}

// joinErrors is a tiny helper to concatenate errors into a single value.
// stdlib's errors.Join is available in Go 1.20+ but we spell it out for clarity.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	msg := "mcp manager: multiple errors:"
	for _, e := range errs {
		msg += "\n  - " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}
