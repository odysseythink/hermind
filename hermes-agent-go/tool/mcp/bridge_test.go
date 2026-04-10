// tool/mcp/bridge_test.go
package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridgeRegistersNamespacedTools(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	reg := tool.NewRegistry()
	b := NewBridge("fs", c, reg)

	// Arrange: tools/list response
	go func() {
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: json.RawMessage(`{
				"tools": [
					{"name":"read_file","description":"Read","inputSchema":{"type":"object"}},
					{"name":"write_file","description":"Write","inputSchema":{"type":"object"}}
				]
			}`),
		})
	}()

	names, err := b.Register(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"fs__read_file", "fs__write_file"}, names)

	// Verify the registry now has both tools
	defs := reg.Definitions(nil)
	assert.Len(t, defs, 2)
}

func TestBridgeHandlerForwardsToolCall(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	reg := tool.NewRegistry()
	b := NewBridge("fs", c, reg)

	// First: tools/list response
	go func() {
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"tools":[{"name":"read_file","description":"R","inputSchema":{}}]}`),
		})
		// Then: tools/call response (triggered by reg.Dispatch below)
		time.Sleep(20 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      2,
			Result:  json.RawMessage(`{"content":[{"type":"text","text":"file contents"}]}`),
		})
	}()

	_, err := b.Register(context.Background())
	require.NoError(t, err)

	out, err := reg.Dispatch(context.Background(), "fs__read_file", json.RawMessage(`{"path":"/tmp/x"}`))
	require.NoError(t, err)
	assert.Equal(t, "file contents", out)
}

func TestBridgeHandlerReturnsServerError(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient(ft)
	require.NoError(t, c.Start(context.Background()))
	defer c.Close()

	reg := tool.NewRegistry()
	b := NewBridge("fs", c, reg)

	go func() {
		time.Sleep(10 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"tools":[{"name":"broken","description":"B","inputSchema":{}}]}`),
		})
		time.Sleep(20 * time.Millisecond)
		ft.injectResponse(&jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      2,
			Result:  json.RawMessage(`{"isError":true,"content":[{"type":"text","text":"something broke"}]}`),
		})
	}()

	_, err := b.Register(context.Background())
	require.NoError(t, err)

	out, err := reg.Dispatch(context.Background(), "fs__broken", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, out, "error")
	assert.Contains(t, out, "something broke")
}

func TestNormalizeSchemaHandlesNull(t *testing.T) {
	assert.Equal(t, json.RawMessage(`{"type":"object"}`), normalizeSchema(nil))
	assert.Equal(t, json.RawMessage(`{"type":"object"}`), normalizeSchema(json.RawMessage("null")))
	assert.Equal(t, json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`),
		normalizeSchema(json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`)))
}
