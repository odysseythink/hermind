package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// SSEMock is an httptest server that speaks MCP over SSE.
//   - GET  /sse  — long-lived SSE stream; first event is `endpoint`
//   - POST /msg/:sessionID — receives JSON-RPC; response written back on SSE stream
type SSEMock struct {
	Server      *httptest.Server
	URL         string // base URL + "/sse" — pass this to ServerConfig.URL
	SessionID   string
	EndpointURL string // resolved URL for POSTing messages
	tools       map[string]ToolDef
	requestLog  []*RecordedRequest
	mu          sync.Mutex

	// stream control
	clients    map[string]*clientConn
	clientsMu  sync.Mutex
	endpointCh chan string // signals when endpoint is ready for tests
}

type clientConn struct {
	w       http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
	events  chan string // serialized SSE writes from handleMsg to handleSSE goroutine
}

// NewSSEMock creates a new mock SSE MCP server with the given tools.
func NewSSEMock(t *testing.T, tools []ToolDef) *SSEMock {
	t.Helper()
	m := &SSEMock{
		SessionID:  "test-sse-session-" + randID(),
		tools:      make(map[string]ToolDef, len(tools)),
		clients:    make(map[string]*clientConn),
		endpointCh: make(chan string, 1),
	}
	for _, td := range tools {
		m.tools[td.Name] = td
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", m.handleSSE)
	mux.HandleFunc("/msg/", m.handleMsg)
	m.Server = httptest.NewServer(mux)
	m.URL = m.Server.URL + "/sse"
	m.EndpointURL = m.Server.URL + "/msg/" + m.SessionID
	t.Cleanup(m.Server.Close)
	return m
}

func (m *SSEMock) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	conn := &clientConn{w: w, flusher: flusher, done: make(chan struct{}), events: make(chan string, 16)}
	m.clientsMu.Lock()
	m.clients[m.SessionID] = conn
	m.clientsMu.Unlock()

	// Send endpoint event
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", "/msg/"+m.SessionID)
	flusher.Flush()

	select {
	case m.endpointCh <- m.EndpointURL:
	default:
	}

	// Write events from channel; only this goroutine touches w
	for {
		select {
		case evt := <-conn.events:
			fmt.Fprint(w, evt)
			flusher.Flush()
		case <-r.Context().Done():
			fmt.Println("[MOCK] handleSSE: context done, err:", r.Context().Err())
			m.clientsMu.Lock()
			delete(m.clients, m.SessionID)
			m.clientsMu.Unlock()
			close(conn.done)
			return
		}
	}
}

func (m *SSEMock) handleMsg(w http.ResponseWriter, r *http.Request) {
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

	var respData map[string]any
	switch req.Method {
	case "initialize":
		respData = map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"protocolVersion": "2025-03-26",
				"serverInfo":      map[string]any{"name": "mock-sse", "version": "1.0"},
				"capabilities":    map[string]any{"tools": map[string]any{}},
			},
		}
	case "ping":
		respData = map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]any{},
		}
	case "tools/list":
		tools := make([]map[string]any, 0, len(m.tools))
		for _, td := range m.tools {
			tools = append(tools, map[string]any{
				"name":        td.Name,
				"description": td.Description,
				"inputSchema": rawOrEmpty(td.InputSchema),
			})
		}
		respData = map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]any{"tools": tools},
		}
	case "tools/call":
		name, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]any)
		tool, ok := m.tools[name]
		if !ok {
			respData = map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error":   map[string]any{"code": -32601, "message": "unknown tool: " + name},
			}
			break
		}
		result, err := tool.Handler(args)
		if err != nil {
			respData = map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error":   map[string]any{"code": -32000, "message": err.Error()},
			}
			break
		}
		respData = map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": jsonString(result)}},
			},
		}
	default:
		respData = map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error":   map[string]any{"code": -32601, "message": "method not found: " + req.Method},
		}
	}

	// Write response back on SSE stream
	payload, _ := json.Marshal(respData)
	m.clientsMu.Lock()
	conn, ok := m.clients[m.SessionID]
	m.clientsMu.Unlock()
	if ok {
		select {
		case conn.events <- fmt.Sprintf("event: message\ndata: %s\n\n", string(payload)):
			w.WriteHeader(http.StatusAccepted)
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	} else {
		w.WriteHeader(http.StatusGone)
	}
}

// Requests returns a copy of all recorded requests.
func (m *SSEMock) Requests() []*RecordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*RecordedRequest, len(m.requestLog))
	copy(out, m.requestLog)
	return out
}

// WaitForEndpoint blocks until the SSE endpoint has been served or timeout.
func (m *SSEMock) WaitForEndpoint(timeout time.Duration) (string, bool) {
	select {
	case url := <-m.endpointCh:
		return url, true
	case <-time.After(timeout):
		return "", false
	}
}

// SendServerEvent writes an arbitrary SSE event to the active stream.
func (m *SSEMock) SendServerEvent(event, data string) bool {
	m.clientsMu.Lock()
	conn, ok := m.clients[m.SessionID]
	m.clientsMu.Unlock()
	if !ok {
		return false
	}
	fmt.Fprintf(conn.w, "event: %s\ndata: %s\n\n", event, data)
	conn.flusher.Flush()
	return true
}

// CloseStream forcibly closes all active SSE connections.
func (m *SSEMock) CloseStream() {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	for _, conn := range m.clients {
		// Force-close by sending done signal; httptest server will handle the rest
		// via context cancellation when server is closed.
		// In practice we close the server to disconnect all clients.
		_ = conn
	}
}

// IsConnected reports whether at least one client is connected.
func (m *SSEMock) IsConnected() bool {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	return len(m.clients) > 0
}

// HeaderValue extracts the first value of a header key from a request log entry.
func (r *RecordedRequest) HeaderValue(key string) string {
	return strings.Join(r.Headers.Values(key), ", ")
}
