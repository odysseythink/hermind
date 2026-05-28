package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// ToolPlugin packages a single MCP-exposed tool for consumption by Go
// agent frameworks. The Call closure dispatches to the live MCP server;
// callers control timeouts and cancellation via the passed-in context.
//
// ToolPlugin values are produced by Hypervisor.ToolsAsPlugins and are
// safe to copy. The Call closure holds an internal reference to the
// Hypervisor, so a plugin obtained while server X was running will
// surface an ErrServerNotFound from Call after X is toggled off.
//
// Note: ToolPlugin.Call does NOT route through the PR-D HTTP-boundary
// concurrency limiter or audit log. Those are concerns of the REST
// route; agent frameworks add their own throttling and observability.
type ToolPlugin struct {
	// ServerName is the configured MCP server name (e.g. "docker-mcp").
	ServerName string
	// ToolName is the tool's name as advertised by the server (e.g. "list-containers").
	ToolName string
	// QualifiedName is "<ServerName>-<ToolName>". Matches Node's plugin naming.
	QualifiedName string
	// Description is the human-readable tool description from the server.
	Description string
	// InputSchema is the JSON Schema for the tool's arguments (may be empty/nil).
	InputSchema json.RawMessage
	// Call invokes the underlying MCP tool with the given arguments. The
	// result is whatever the MCP server returned (typically an object with
	// `content` array). The caller controls timeout/cancellation via ctx.
	Call func(ctx context.Context, args map[string]any) (any, error)
}

// ActiveServers returns the list of currently-running MCP servers tagged
// with the "@@mcp_" prefix that agent frameworks use to mark MCP-sourced
// plugin sets in their plugin config arrays. The list is sorted
// lexicographically by server name for deterministic agent behaviour.
//
// A server appears in the list iff it is present in the running mcps
// map — failed boots, autoStart=false servers, and pruned servers are
// all excluded.
func (h *Hypervisor) ActiveServers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.mcps))
	for name := range h.mcps {
		out = append(out, "@@mcp_"+name)
	}
	sort.Strings(out)
	return out
}

// ToolsAsPlugins returns each non-suppressed tool on the named running
// server as a ToolPlugin. The Call closure on each plugin dispatches to
// the underlying MCP server.
//
// Returns an error wrapping ErrServerNotFound if the server is not
// currently running. Returns ([]ToolPlugin{}, nil) — non-nil, zero-length —
// if the server is running but every tool is suppressed.
func (h *Hypervisor) ToolsAsPlugins(name string) ([]ToolPlugin, error) {
	h.mu.RLock()
	client, ok := h.mcps[name]
	// Capture the data we need under the lock; build the slice without it.
	var tools []ToolSchema
	if ok {
		tools = append(tools, client.tools...)
	}
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrServerNotFound, name)
	}

	suppressed := h.suppressionSetForServer(name)
	out := make([]ToolPlugin, 0, len(tools))
	for i := range tools {
		t := tools[i] // copy — critical for closure capture; see Task 3
		if _, blocked := suppressed[t.Name]; blocked {
			continue
		}
		server, tool := name, t.Name // explicit copy into closure scope
		call := func(ctx context.Context, args map[string]any) (any, error) {
			return h.CallTool(ctx, server, tool, args)
		}
		out = append(out, ToolPlugin{
			ServerName:    server,
			ToolName:      tool,
			QualifiedName: server + "-" + tool,
			Description:   t.Description,
			InputSchema:   t.InputSchema,
			Call:          call,
		})
	}
	return out, nil
}

// suppressionSetForServer reads the per-server suppressed-tool list from
// the JSON config file. Returns an empty (non-nil) set on miss.
func (h *Hypervisor) suppressionSetForServer(name string) map[string]struct{} {
	list := h.file.GetSuppressedTools(name)
	set := make(map[string]struct{}, len(list))
	for _, s := range list {
		set[s] = struct{}{}
	}
	return set
}
