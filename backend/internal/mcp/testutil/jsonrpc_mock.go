package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// ToolDef defines a mock tool that the HTTP mock can register and invoke.
type ToolDef struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	// Handler is called when client invokes this tool.
	Handler func(args map[string]any) (any, error)
}

// RecordedRequest captures one JSON-RPC request received by the mock.
type RecordedRequest struct {
	Method  string
	Params  map[string]any
	Headers http.Header
}

// StreamableHTTPMock is an httptest server that speaks MCP JSON-RPC over HTTP.
type StreamableHTTPMock struct {
	Server     *httptest.Server
	URL        string
	SessionID  string
	tools      map[string]ToolDef
	requestLog []*RecordedRequest
	mu         sync.Mutex
}

// NewStreamableHTTPMock creates a new mock HTTP MCP server with the given tools.
func NewStreamableHTTPMock(t *testing.T, tools []ToolDef) *StreamableHTTPMock {
	t.Helper()
	m := &StreamableHTTPMock{
		SessionID: "test-session-" + randID(),
		tools:     make(map[string]ToolDef, len(tools)),
	}
	for _, td := range tools {
		m.tools[td.Name] = td
	}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	m.URL = m.Server.URL
	t.Cleanup(m.Server.Close)
	return m
}

func (m *StreamableHTTPMock) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  map[string]any  `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	m.mu.Lock()
	m.requestLog = append(m.requestLog, &RecordedRequest{
		Method:  req.Method,
		Params:  req.Params,
		Headers: r.Header.Clone(),
	})
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Mcp-Session-Id", m.SessionID)

	switch req.Method {
	case "initialize":
		writeJSONRPCResult(w, req.ID, map[string]any{
			"protocolVersion": "2025-03-26",
			"serverInfo":      map[string]any{"name": "mock-http", "version": "1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		})
	case "ping":
		writeJSONRPCResult(w, req.ID, map[string]any{})
	case "tools/list":
		tools := make([]map[string]any, 0, len(m.tools))
		for _, td := range m.tools {
			tools = append(tools, map[string]any{
				"name":        td.Name,
				"description": td.Description,
				"inputSchema": rawOrEmpty(td.InputSchema),
			})
		}
		writeJSONRPCResult(w, req.ID, map[string]any{"tools": tools})
	case "tools/call":
		name, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]any)
		tool, ok := m.tools[name]
		if !ok {
			writeJSONRPCError(w, req.ID, -32601, "unknown tool: "+name)
			return
		}
		result, err := tool.Handler(args)
		if err != nil {
			writeJSONRPCError(w, req.ID, -32000, err.Error())
			return
		}
		writeJSONRPCResult(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": jsonString(result)}},
		})
	default:
		writeJSONRPCError(w, req.ID, -32601, "method not found: "+req.Method)
	}
}

// Requests returns a copy of all recorded requests.
func (m *StreamableHTTPMock) Requests() []*RecordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*RecordedRequest, len(m.requestLog))
	copy(out, m.requestLog)
	return out
}

func writeJSONRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func jsonString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func rawOrEmpty(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage("{}")
	}
	return r
}

func randID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
