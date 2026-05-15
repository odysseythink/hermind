// tool/mcp/bridge.go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// Bridge registers an MCP client's tools into a hermes tool.Registry.
// Tools are namespaced as "serverName__toolName" to prevent collisions
// when multiple servers expose the same tool name.
type Bridge struct {
	serverName string
	client     *Client
	registry   *tool.Registry

	registeredNames []string
}

// NewBridge creates a Bridge for the given server name, client, and registry.
func NewBridge(serverName string, client *Client, registry *tool.Registry) *Bridge {
	return &Bridge{
		serverName: serverName,
		client:     client,
		registry:   registry,
	}
}

// Register fetches the tool list from the server and registers each tool
// in the registry. Returns the list of registered tool names.
func (b *Bridge) Register(ctx context.Context) ([]string, error) {
	tools, err := b.client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("bridge %s: list tools: %w", b.serverName, err)
	}

	for _, t := range tools {
		registered := b.registerOne(t)
		if registered != "" {
			b.registeredNames = append(b.registeredNames, registered)
		}
	}

	return b.registeredNames, nil
}

// registerOne converts a single MCP Tool into a hermes tool.Entry
// and registers it. Returns the hermes tool name that was registered.
func (b *Bridge) registerOne(mcpTool Tool) string {
	if mcpTool.Name == "" {
		return ""
	}
	hermesName := b.serverName + "__" + mcpTool.Name

	entry := &tool.Entry{
		Name:        hermesName,
		Toolset:     "mcp",
		Description: mcpTool.Description,
		Emoji:       "🔌",
		Schema: core.ToolDefinition{
			Name:        hermesName,
			Description: mcpTool.Description,
			Parameters:  normalizeSchema(mcpTool.InputSchema),
		},
		Handler: b.makeHandler(mcpTool.Name),
	}

	b.registry.Register(entry)
	return hermesName
}

// normalizeSchema ensures the input schema is a valid JSON object.
// Some MCP servers return an empty schema as null or empty string.
func normalizeSchema(raw json.RawMessage) *core.Schema {
	if len(raw) == 0 || string(raw) == "null" {
		return core.MustSchemaFromJSON([]byte(`{"type":"object"}`))
	}
	return core.MustSchemaFromJSON(raw)
}

// makeHandler returns a tool.Handler that forwards the call to the MCP server.
func (b *Bridge) makeHandler(mcpToolName string) tool.Handler {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		resp, err := b.client.CallTool(ctx, mcpToolName, args)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("mcp %s: %s", b.serverName, err.Error())), nil
		}
		if resp.IsError {
			return tool.ToolError("mcp server returned error: " + FlattenContent(resp)), nil
		}
		text := FlattenContent(resp)
		if text == "" {
			return tool.ToolResult(map[string]any{"ok": true}), nil
		}
		// MCP tools typically return text (often JSON) — pass through.
		return text, nil
	}
}

// Unregister is a no-op placeholder. Plan 6b does not support dynamic
// tool removal (the tool.Registry API does not expose a Remove method).
// Plan 6c could add that capability.
func (b *Bridge) Unregister() []string {
	names := b.registeredNames
	// We don't actually remove from the registry — returning the names lets
	// the Manager log what would have been unregistered.
	return names
}

// sanitize is a small helper kept for potential future name cleaning.
func sanitize(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}
