package mcp

import "errors"

// ErrorCode is a stable identifier for tool-call failures returned to clients.
type ErrorCode string

const (
	CodeInvalidBody        ErrorCode = "INVALID_BODY"
	CodeInvalidParams      ErrorCode = "INVALID_PARAMS"
	CodeServerNotFound     ErrorCode = "SERVER_NOT_FOUND"
	CodeToolNotFound       ErrorCode = "TOOL_NOT_FOUND"
	CodeArgsSchemaMismatch ErrorCode = "ARGS_SCHEMA_MISMATCH"
	CodeBodyTooLarge       ErrorCode = "BODY_TOO_LARGE"
	CodeConcurrencyLimit   ErrorCode = "CONCURRENCY_LIMIT"
	CodeCallTimeout        ErrorCode = "CALL_TIMEOUT"
	CodeTransportError     ErrorCode = "TRANSPORT_ERROR"
	CodeInternalError      ErrorCode = "INTERNAL_ERROR"
)

// ErrToolNotFound is returned by Hypervisor.GetToolSchema when the named
// tool is not present in the running server's tool list.
var ErrToolNotFound = errors.New("MCP tool not found")
