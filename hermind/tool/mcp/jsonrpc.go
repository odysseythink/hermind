// tool/mcp/jsonrpc.go
package mcp

import "encoding/json"

// JSON-RPC 2.0 message types.
// Reference: https://www.jsonrpc.org/specification
//
// The MCP protocol layers on top of JSON-RPC 2.0 — MCP-specific methods
// like "initialize", "tools/list", "tools/call" are dispatched via the
// standard JSON-RPC request/response flow.

// jsonrpcRequest is a client->server call that expects a response.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`          // always "2.0"
	ID      int64           `json:"id"`               // client-assigned correlation ID
	Method  string          `json:"method"`           // method name, e.g. "tools/list"
	Params  json.RawMessage `json:"params,omitempty"` // optional params
}

// jsonrpcNotification is a client->server call that does NOT expect a response.
// Notifications have no ID.
type jsonrpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse is the server's response to a request.
// Exactly one of Result or Error is populated (never both).
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// jsonrpcError is a JSON-RPC error object.
type jsonrpcError struct {
	Code    int             `json:"code"`    // -32700 parse, -32600 invalid req, -32601 method not found, ...
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface for jsonrpcError.
func (e *jsonrpcError) Error() string {
	return e.Message
}

// newRequest constructs a JSON-RPC request with the given ID, method, and params.
// params may be nil.
func newRequest(id int64, method string, params any) (*jsonrpcRequest, error) {
	req := &jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		req.Params = data
	}
	return req, nil
}

// newNotification constructs a JSON-RPC notification (no response expected).
func newNotification(method string, params any) (*jsonrpcNotification, error) {
	n := &jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  method,
	}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		n.Params = data
	}
	return n, nil
}
