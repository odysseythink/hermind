// Package mcp implements the MCP hypervisor: lifecycle, config, and transport abstraction.
// PR-A ships skeleton + REST only; transports land in PR-B/C.
//
// # Agent plugin surface
//
// The hypervisor exposes ActiveServers() and ToolsAsPlugins(name) so a
// future Go agent framework can discover and invoke MCP-backed tools.
//
// ActiveServers returns ["@@mcp_<name>", ...] identifiers — the same
// convention Node's aibitat uses to mark "expand this into all of that
// server's tools" entries in a plugin list.
//
// ToolsAsPlugins(name) returns one server's non-suppressed tools as
// ToolPlugin values. Each ToolPlugin carries the tool's name,
// description, input schema, and a Call closure that dispatches to the
// live MCP server.
//
// ToolPlugin.Call does NOT route through the REST handler's HTTP-boundary
// concurrency limiter or audit logger from PR-D — those are for the
// /api/mcp/.../tools/.../call route only. Agent frameworks should add
// their own throttling and observability if desired.
//
// The plugin surface is in-process Go API only. No new REST endpoints.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrTransportNotImplemented = errors.New("MCP transport not implemented")
	ErrInvalidServerType       = errors.New("MCP server type is invalid")
	ErrServerNotFound          = errors.New("MCP server not found in config file")
	ErrServerNameRequired      = errors.New("MCP server name is required")
)

type ServerConfig struct {
	Name        string              `json:"-"`
	Command     string              `json:"command,omitempty"`
	Args        []string            `json:"args,omitempty"`
	Env         map[string]string   `json:"env,omitempty"`
	URL         string              `json:"url,omitempty"`
	Type        string              `json:"type,omitempty"`
	Headers     map[string]string   `json:"headers,omitempty"`
	AnythingLLM *AnythingLLMOptions `json:"anythingllm,omitempty"`
}

// SuppressedTools returns the suppressed tool names for this server, nil-safe.
func (c *ServerConfig) SuppressedTools() []string {
	if c.AnythingLLM == nil {
		return nil
	}
	return c.AnythingLLM.SuppressedTools
}

type AnythingLLMOptions struct {
	AutoStart       *bool    `json:"autoStart,omitempty"`
	SuppressedTools []string `json:"suppressedTools,omitempty"`
	MaxConcurrency  *int     `json:"maxConcurrency,omitempty"`
}

type ServerView struct {
	Name    string        `json:"name"`
	Config  *ServerConfig `json:"config"`
	Running bool          `json:"running"`
	Tools   []ToolSchema  `json:"tools"`
	Error   *string       `json:"error"`
	Process *ProcessInfo  `json:"process"`
}

type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ProcessInfo struct {
	PID int    `json:"pid"`
	Cmd string `json:"cmd,omitempty"`
}

type LoadResult struct {
	Status  string // "success" | "failed"
	Message string
}

// Transport defines the interface for MCP server transports (stdio, HTTP, SSE).
// Real implementations land in PR-B (stdio) and PR-C (HTTP/SSE).
type Transport interface {
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) bool
	ListTools(ctx context.Context) ([]ToolSchema, error)
	CallTool(ctx context.Context, name string, args map[string]any) (any, error)
	ProcessInfo() *ProcessInfo
}
