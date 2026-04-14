// tool/mcp/protocol.go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// MCP protocol version implemented by this client.
// Reference: https://modelcontextprotocol.io/specification
const protocolVersion = "2024-11-05"

// ClientInfo identifies this client to MCP servers during initialize.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeRequest is sent as the first call to any MCP server.
type InitializeRequest struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      ClientInfo     `json:"clientInfo"`
}

// InitializeResponse is the server's response to initialize.
type InitializeResponse struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

// Tool is a single tool exposed by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ListToolsResponse is the response to tools/list.
type ListToolsResponse struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// CallToolRequest is the request for tools/call.
type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResponse is the response to tools/call.
type CallToolResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is one element of a CallToolResponse content array.
// MCP supports multiple content types; Plan 6b handles "text" only.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Initialize performs the MCP handshake:
//  1. Send an "initialize" request with our protocol version and client info.
//  2. Read the server's response (protocol version, capabilities, server info).
//  3. Send an "initialized" notification to confirm.
//
// After a successful Initialize, the client can call tools/list and tools/call.
func (c *Client) Initialize(ctx context.Context, clientName, clientVersion string) (*InitializeResponse, error) {
	req := InitializeRequest{
		ProtocolVersion: protocolVersion,
		Capabilities:    map[string]any{}, // Plan 6b advertises no client capabilities
		ClientInfo: ClientInfo{
			Name:    clientName,
			Version: clientVersion,
		},
	}

	raw, err := c.Call(ctx, "initialize", req)
	if err != nil {
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}

	var resp InitializeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("mcp initialize: decode: %w", err)
	}

	// Stash server info for later callers
	c.serverName = resp.ServerInfo.Name
	c.serverVersion = resp.ServerInfo.Version

	// Send the "initialized" notification to complete the handshake.
	if err := c.Notify("notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("mcp initialize: notify: %w", err)
	}

	return &resp, nil
}

// ListTools fetches the list of tools exposed by the server.
// Plan 6b does not paginate — it assumes all tools fit in a single response.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.Call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}

	var resp ListToolsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("mcp tools/list: decode: %w", err)
	}
	return resp.Tools, nil
}

// CallTool invokes a tool on the server with the given arguments.
// Arguments are passed as a JSON object — pass nil if the tool takes no args.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (*CallToolResponse, error) {
	req := CallToolRequest{
		Name:      name,
		Arguments: args,
	}

	raw, err := c.Call(ctx, "tools/call", req)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/call %s: %w", name, err)
	}

	var resp CallToolResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("mcp tools/call %s: decode: %w", name, err)
	}
	return &resp, nil
}

// FlattenContent concatenates text blocks from a CallToolResponse into a single
// string. Non-text blocks are dropped. This matches the existing hermes tool
// handler signature which returns a single string.
func FlattenContent(resp *CallToolResponse) string {
	if resp == nil {
		return ""
	}
	var out string
	for _, b := range resp.Content {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return out
}
