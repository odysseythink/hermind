// tool/mcp/protocol_test.go
package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeHandshake(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	// Inject the initialize response
	go func() {
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test-server","version":"1.0.0"}}`),
		})
	}()

	resp, err := c.Initialize(context.Background(), "hermes", "0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "2024-11-05", resp.ProtocolVersion)
	assert.Equal(t, "test-server", resp.ServerInfo.Name)

	// Verify ServerInfo is stashed
	name, version := c.ServerInfo()
	assert.Equal(t, "test-server", name)
	assert.Equal(t, "1.0.0", version)

	// Verify the "initialized" notification was sent.
	// Sent slice should have: 1) initialize request, 2) initialized notification
	ft.mu.Lock()
	defer ft.mu.Unlock()
	require.GreaterOrEqual(t, len(ft.sent), 2)
}

func TestListToolsHappyPath(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: json.RawMessage(`{
				"tools": [
					{"name":"read_file","description":"Read a file","inputSchema":{"type":"object"}},
					{"name":"write_file","description":"Write a file","inputSchema":{"type":"object"}}
				]
			}`),
		})
	}()

	tools, err := c.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Equal(t, "read_file", tools[0].Name)
	assert.Equal(t, "write_file", tools[1].Name)
}

func TestCallToolHappyPath(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: json.RawMessage(`{
				"content": [
					{"type":"text","text":"hello from tool"}
				]
			}`),
		})
	}()

	resp, err := c.CallTool(context.Background(), "read_file", json.RawMessage(`{"path":"/tmp/x"}`))
	require.NoError(t, err)
	assert.Len(t, resp.Content, 1)
	assert.Equal(t, "hello from tool", resp.Content[0].Text)
	assert.False(t, resp.IsError)
}

func TestCallToolFlattensContent(t *testing.T) {
	resp := &CallToolResponse{
		Content: []ContentBlock{
			{Type: "text", Text: "hello "},
			{Type: "text", Text: "world"},
			{Type: "image", Text: "ignored"},
		},
	}
	assert.Equal(t, "hello world", FlattenContent(resp))
}
