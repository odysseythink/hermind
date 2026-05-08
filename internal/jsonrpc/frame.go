// Package jsonrpc implements the minimal subset of JSON-RPC 2.0 used
// by hermind's stdio servers (acp/stdio and mcp/server). Frames are
// newline-delimited; Content-Length headers are not supported.
package jsonrpc

import (
	"encoding/json"
	"fmt"
	"io"
)

const Version = "2.0"

// Standard error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Request is a decoded JSON-RPC 2.0 request.
type Request struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification reports whether this request carries no ID.
func (r *Request) IsNotification() bool {
	return len(r.ID) == 0 || string(r.ID) == "null"
}

// Response is a JSON-RPC 2.0 response frame.
type Response struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is the JSON-RPC error object.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// DecodeRequest parses a single newline-delimited frame.
func DecodeRequest(raw []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("jsonrpc: decode: %w", err)
	}
	if req.Version != "" && req.Version != Version {
		return nil, fmt.Errorf("jsonrpc: unsupported version %q", req.Version)
	}
	return &req, nil
}

// EncodeResponse writes resp + "\n".
func EncodeResponse(w io.Writer, resp *Response) error {
	resp.Version = Version
	if resp.Result == nil && resp.Error == nil {
		resp.Result = json.RawMessage(`null`)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

// EncodeNotification writes a notification frame (no id).
func EncodeNotification(w io.Writer, method string, params interface{}) error {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Version string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{Version, method, rawParams})
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}
