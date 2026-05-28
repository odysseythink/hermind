package services

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/mcp"
)

type MCPService struct {
	hv *mcp.Hypervisor
}

func NewMCPService(hv *mcp.Hypervisor) *MCPService {
	return &MCPService{hv: hv}
}

func (s *MCPService) Servers(ctx context.Context) ([]mcp.ServerView, error) {
	return s.hv.Servers(ctx)
}

func (s *MCPService) Reload(ctx context.Context) ([]mcp.ServerView, error) {
	return s.hv.Reload(ctx)
}

func (s *MCPService) ToggleServer(ctx context.Context, name string) (bool, error) {
	return s.hv.ToggleServer(ctx, name)
}

func (s *MCPService) DeleteServer(ctx context.Context, name string) (bool, error) {
	return s.hv.DeleteServer(ctx, name)
}

func (s *MCPService) ToggleTool(ctx context.Context, serverName, toolName string, enabled bool) ([]string, error) {
	return s.hv.ToggleTool(ctx, serverName, toolName, enabled)
}

func (s *MCPService) GetToolSchema(server, tool string) (*mcp.ToolSchema, error) {
	return s.hv.GetToolSchema(server, tool)
}

func (s *MCPService) TryAcquireCall(server string) bool { return s.hv.TryAcquireCall(server) }
func (s *MCPService) ReleaseCall(server string)         { s.hv.ReleaseCall(server) }

func (s *MCPService) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error) {
	return s.hv.CallTool(ctx, serverName, toolName, args)
}

// ActiveServers returns "@@mcp_<name>" identifiers for each running MCP
// server. Intended for consumption by a Go agent framework's plugin
// resolver. See mcp.Hypervisor.ActiveServers for full semantics.
func (s *MCPService) ActiveServers() []string {
	return s.hv.ActiveServers()
}

// ToolsAsPlugins returns each non-suppressed tool on the named running
// server as an mcp.ToolPlugin. Returns an error wrapping
// mcp.ErrServerNotFound if the server is not running.
func (s *MCPService) ToolsAsPlugins(name string) ([]mcp.ToolPlugin, error) {
	return s.hv.ToolsAsPlugins(name)
}
