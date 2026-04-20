// Package stdio implements the Agent Client Protocol (ACP) over
// newline-delimited JSON-RPC 2.0 on stdin/stdout. An editor such as
// Zed, Cursor or VS Code spawns `hermind acp` and drives the agent by
// writing request frames to its stdin and reading response frames
// from its stdout.
//
// This package reuses the shared codec in internal/jsonrpc for
// wire framing and adds ACP-specific request/response payload types
// plus the method handlers and the read/dispatch loop.
package stdio

import (
	"encoding/json"
	"io"

	"github.com/odysseythink/hermind/internal/jsonrpc"
)

// Re-exports so callers (and tests) inside this package don't need a
// second import of internal/jsonrpc for the bread-and-butter types.
type (
	Request  = jsonrpc.Request
	Response = jsonrpc.Response
	Error    = jsonrpc.Error
)

// Standard JSON-RPC 2.0 error codes, re-exported for convenience.
const (
	CodeParseError     = jsonrpc.CodeParseError
	CodeInvalidRequest = jsonrpc.CodeInvalidRequest
	CodeMethodNotFound = jsonrpc.CodeMethodNotFound
	CodeInvalidParams  = jsonrpc.CodeInvalidParams
	CodeInternalError  = jsonrpc.CodeInternalError
)

// DecodeRequest parses a single JSON-RPC frame.
func DecodeRequest(raw []byte) (*Request, error) {
	return jsonrpc.DecodeRequest(raw)
}

// EncodeResponse writes resp followed by a single newline. The jsonrpc
// version field is stamped automatically.
func EncodeResponse(w io.Writer, resp *Response) error {
	return jsonrpc.EncodeResponse(w, resp)
}

// EncodeNotification writes a server-initiated notification frame.
func EncodeNotification(w io.Writer, method string, params interface{}) error {
	return jsonrpc.EncodeNotification(w, method, params)
}

// ---- ACP payload shapes ----

// initializeResult is returned from the `initialize` method. Fields
// mirror the Zed ACP spec shape accepted by their reference client.
type initializeResult struct {
	ProtocolVersion int          `json:"protocolVersion"`
	AgentInfo       agentInfo    `json:"agentInfo"`
	AgentCapability agentCap     `json:"agentCapabilities"`
	AuthMethods     []authMethod `json:"authMethods,omitempty"`
}

type agentInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type agentCap struct {
	LoadSession bool `json:"loadSession"`
}

type authMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// newSessionParams is the `session/new` request payload.
type newSessionParams struct {
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers,omitempty"`
	Model      string            `json:"model,omitempty"`
}

// newSessionResult is the `session/new` response payload.
type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

// loadSessionParams is the `session/load` request payload.
type loadSessionParams struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd,omitempty"`
}

// promptContentBlock is one entry in a `session/prompt` prompt array.
// Only text blocks are handled in the MVP; other block types are
// decoded for completeness and ignored.
type promptContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// promptParams is the `session/prompt` request payload.
type promptParams struct {
	SessionID string               `json:"sessionId"`
	Prompt    []promptContentBlock `json:"prompt"`
}

// promptResult is the `session/prompt` response payload.
type promptResult struct {
	StopReason string `json:"stopReason"`
}

// cancelParams is the `session/cancel` request payload.
type cancelParams struct {
	SessionID string `json:"sessionId"`
}
