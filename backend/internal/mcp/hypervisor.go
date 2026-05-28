package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
)

const connectionTimeout = 30 * time.Second

// connectionTimeoutForTest allows tests to override the 30s boot timeout.
// Set to >0 before a test and reset in t.Cleanup.
var connectionTimeoutForTest = 0 * time.Second

func effectiveConnectionTimeout() time.Duration {
	if connectionTimeoutForTest > 0 {
		return connectionTimeoutForTest
	}
	return connectionTimeout
}

type Hypervisor struct {
	mu      sync.RWMutex
	cfg     *config.Config
	file    *Config
	mcps    map[string]*activeClient
	results map[string]LoadResult
	limiter *concurrencyLimiter
}

type activeClient struct {
	transport    Transport
	process      *ProcessInfo
	tools        []ToolSchema
	schemaByName map[string]json.RawMessage
}

func newHypervisor(cfg *config.Config) *Hypervisor {
	return &Hypervisor{
		cfg:     cfg,
		file:    NewConfig(cfg.StorageDir),
		mcps:    make(map[string]*activeClient),
		results: make(map[string]LoadResult),
		limiter: newConcurrencyLimiter(cfg.MCPCallConcurrencyPerServer),
	}
}

// NewHypervisorForTesting bypasses the singleton for test isolation.
// Do not call from production code.
func NewHypervisorForTesting(cfg *config.Config) *Hypervisor {
	return newHypervisor(cfg)
}

func (h *Hypervisor) Boot(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.mcps) > 0 {
		return nil // already booted
	}
	if err := h.file.Ensure(); err != nil {
		return err
	}
	servers, err := h.file.Load()
	if err != nil {
		return err
	}
	for i := range servers {
		srv := &servers[i]
		if srv.Hermind != nil && srv.Hermind.AutoStart != nil && !*srv.Hermind.AutoStart {
			h.results[srv.Name] = LoadResult{
				Status:  "failed",
				Message: fmt.Sprintf("MCP server %s has hermind.autoStart=false, boot skipped", srv.Name),
			}
			continue
		}
		h.startServerLocked(ctx, srv)
	}
	return nil
}

func (h *Hypervisor) startServerLocked(parent context.Context, srv *ServerConfig) {
	ctx, cancel := context.WithTimeout(parent, effectiveConnectionTimeout())
	defer cancel()
	transport, err := newTransport(srv)
	if err != nil {
		h.results[srv.Name] = LoadResult{Status: "failed", Message: err.Error()}
		return
	}
	if err := transport.Connect(ctx); err != nil {
		_ = transport.Close()
		h.results[srv.Name] = LoadResult{Status: "failed", Message: err.Error()}
		return
	}
	tools, err := transport.ListTools(ctx)
	if err != nil {
		// Tools list failure is non-fatal — server is up but schema cache stays empty.
		// Operator can refresh by toggling.
		mlog.Warning("mcp tool list failed", mlog.String("server", srv.Name), mlog.Err(err))
		tools = nil
	}
	schemaByName := make(map[string]json.RawMessage, len(tools))
	for _, t := range tools {
		if len(t.InputSchema) > 0 {
			schemaByName[t.Name] = t.InputSchema
		}
	}
	h.mcps[srv.Name] = &activeClient{
		transport:    transport,
		process:      transport.ProcessInfo(),
		tools:        tools,
		schemaByName: schemaByName,
	}
	if srv.Hermind != nil && srv.Hermind.MaxConcurrency != nil {
		h.limiter.SetOverride(srv.Name, *srv.Hermind.MaxConcurrency)
	}
	h.results[srv.Name] = LoadResult{Status: "success", Message: fmt.Sprintf("Successfully connected to MCP server: %s", srv.Name)}
}

func (h *Hypervisor) Servers(ctx context.Context) ([]ServerView, error) {
	if err := h.file.Ensure(); err != nil {
		return nil, err
	}
	configs, err := h.file.Load()
	if err != nil {
		return nil, err
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]ServerView, 0, len(configs))
	for i := range configs {
		srv := configs[i]
		view := ServerView{Name: srv.Name, Config: &srv, Tools: []ToolSchema{}}
		result, hasResult := h.results[srv.Name]
		client, hasClient := h.mcps[srv.Name]
		switch {
		case hasResult && result.Status == "failed":
			msg := result.Message
			view.Error = &msg
			view.Running = false
		case hasClient:
			online := client.transport.Ping(ctx)
			view.Running = online
			view.Process = client.process
			if online {
				suppressed := stringSet(srv.SuppressedTools())
				for _, t := range client.tools {
					if _, ok := suppressed[t.Name]; !ok {
						view.Tools = append(view.Tools, t)
					}
				}
			}
		default:
			// Never booted (e.g. cold list before Boot)
			transportErr := ErrTransportNotImplemented.Error()
			view.Error = &transportErr
		}
		out = append(out, view)
	}
	return out, nil
}

func (h *Hypervisor) Reload(ctx context.Context) ([]ServerView, error) {
	return h.Servers(ctx)
}

func (h *Hypervisor) ToggleServer(ctx context.Context, name string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	servers, err := h.file.Load()
	if err != nil {
		return false, err
	}
	var found *ServerConfig
	for i := range servers {
		if servers[i].Name == name {
			found = &servers[i]
			break
		}
	}
	if found == nil {
		return false, fmt.Errorf("%w: %s", ErrServerNotFound, name)
	}
	if _, on := h.mcps[name]; on {
		h.pruneServerLocked(name)
		return true, nil
	}
	h.startServerLocked(ctx, found)
	result := h.results[name]
	if result.Status != "success" {
		return false, errors.New(result.Message)
	}
	return true, nil
}

func (h *Hypervisor) DeleteServer(ctx context.Context, name string) (bool, error) {
	h.mu.Lock()
	h.pruneServerLocked(name)
	h.mu.Unlock()
	ok, err := h.file.RemoveServer(name)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("%w: %s", ErrServerNotFound, name)
	}
	return true, nil
}

func (h *Hypervisor) ToggleTool(ctx context.Context, serverName, toolName string, enabled bool) ([]string, error) {
	return h.file.UpdateSuppressedTools(serverName, toolName, enabled)
}

// GetToolSchema returns the cached tool definition for (server, tool). The
// returned ToolSchema may have a zero-length InputSchema if the upstream
// server didn't advertise one; callers should treat that as "skip
// validation" rather than as an error.
func (h *Hypervisor) GetToolSchema(server, tool string) (*ToolSchema, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.mcps[server]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrServerNotFound, server)
	}
	for i := range client.tools {
		if client.tools[i].Name == tool {
			t := client.tools[i]
			return &t, nil
		}
	}
	return nil, fmt.Errorf("%w: %s on server %s", ErrToolNotFound, tool, server)
}

func (h *Hypervisor) TryAcquireCall(server string) bool { return h.limiter.TryAcquire(server) }
func (h *Hypervisor) ReleaseCall(server string)         { h.limiter.Release(server) }

func (h *Hypervisor) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error) {
	h.mu.RLock()
	client, ok := h.mcps[serverName]
	h.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s (not running)", ErrServerNotFound, serverName)
	}
	return client.transport.CallTool(ctx, toolName, args)
}

func (h *Hypervisor) PruneAll() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for name := range h.mcps {
		h.pruneServerLocked(name)
	}
	return nil
}

func (h *Hypervisor) pruneServerLocked(name string) {
	client, ok := h.mcps[name]
	if !ok {
		return
	}
	_ = client.transport.Close()
	delete(h.mcps, name)
	h.limiter.ClearOverride(name)
	h.results[name] = LoadResult{Status: "failed", Message: "Server was stopped manually by the administrator."}
}

// parseServerType inspects a ServerConfig and returns one of "stdio" | "http" | "sse".
// Matches Node's #parseServerType behavior.
func parseServerType(srv *ServerConfig) string {
	switch srv.Type {
	case "streamable", "http":
		return "http"
	case "sse":
		return "sse"
	}
	if srv.Command != "" {
		return "stdio"
	}
	if srv.URL != "" {
		return "http"
	}
	return "sse"
}

func validateServerDefinition(name string, srv *ServerConfig, kind string) error {
	switch srv.Type {
	case "sse", "streamable", "http":
		if srv.URL == "" {
			return fmt.Errorf("MCP server %q: missing required %q for %s transport", name, "url", srv.Type)
		}
		if _, err := url.Parse(srv.URL); err != nil || !isValidURL(srv.URL) {
			return fmt.Errorf("MCP server %q: invalid URL %q", name, srv.URL)
		}
		return nil
	}
	switch kind {
	case "stdio":
		return nil // Args type-checked by Go
	case "http":
		if srv.Type != "sse" && srv.Type != "streamable" {
			return fmt.Errorf("MCP server type must be sse or streamable")
		}
	}
	return nil
}

func isValidURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func stringSet(xs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}
