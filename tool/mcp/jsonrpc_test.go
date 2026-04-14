// tool/mcp/jsonrpc_test.go
package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequestWithParams(t *testing.T) {
	req, err := newRequest(42, "tools/call", map[string]any{"name": "read_file", "arguments": map[string]any{"path": "/tmp/x"}})
	require.NoError(t, err)
	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, int64(42), req.ID)
	assert.Equal(t, "tools/call", req.Method)
	assert.Contains(t, string(req.Params), `"name":"read_file"`)
}

func TestNewRequestNilParams(t *testing.T) {
	req, err := newRequest(1, "tools/list", nil)
	require.NoError(t, err)
	assert.Nil(t, req.Params)
}

func TestNewNotification(t *testing.T) {
	n, err := newNotification("notifications/initialized", nil)
	require.NoError(t, err)
	assert.Equal(t, "2.0", n.JSONRPC)
	assert.Equal(t, "notifications/initialized", n.Method)
	assert.Nil(t, n.Params)
}

func TestResponseUnmarshal(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":7,"result":{"tools":[]}}`)
	var resp jsonrpcResponse
	require.NoError(t, json.Unmarshal(raw, &resp))
	assert.Equal(t, int64(7), resp.ID)
	assert.Nil(t, resp.Error)
	assert.Contains(t, string(resp.Result), "tools")
}

func TestResponseWithError(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"method not found"}}`)
	var resp jsonrpcResponse
	require.NoError(t, json.Unmarshal(raw, &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "method not found", resp.Error.Error())
}
