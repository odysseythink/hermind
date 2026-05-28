package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamableHTTPMock_InitializeRoundtrip(t *testing.T) {
	m := NewStreamableHTTPMock(t, nil)

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(m.URL, "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, m.SessionID, resp.Header.Get("Mcp-Session-Id"))

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "2.0", result["jsonrpc"])
	assert.NotNil(t, result["result"])
}

func TestStreamableHTTPMock_RecordsRequests(t *testing.T) {
	m := NewStreamableHTTPMock(t, nil)

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(m.URL, "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	resp.Body.Close()

	reqs := m.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "tools/list", reqs[0].Method)
}

func TestStreamableHTTPMock_ToolCallRoundtrip(t *testing.T) {
	m := NewStreamableHTTPMock(t, []ToolDef{
		{
			Name:        "echo",
			Description: "echoes text",
			Handler: func(args map[string]any) (any, error) {
				return args["text"], nil
			},
		},
	})

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "echo",
			"arguments": map[string]any{"text": "hello"},
		},
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(m.URL, "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotNil(t, result["result"])
	res := result["result"].(map[string]any)
	content := res["content"].([]any)
	require.Len(t, content, 1)
	assert.Equal(t, "hello", content[0].(map[string]any)["text"])
}

func TestSSEMock_EndpointEventFirst(t *testing.T) {
	m := NewSSEMock(t, nil)

	// The mock server runs on httptest; we can directly GET the SSE endpoint.
	req, err := http.NewRequest(http.MethodGet, m.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Read first SSE event
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	require.NoError(t, err)
	event := string(buf[:n])
	assert.Contains(t, event, "event: endpoint")
	assert.Contains(t, event, "/msg/"+m.SessionID)
}

func TestSSEMock_ToolCallRoundtrip(t *testing.T) {
	m := NewSSEMock(t, []ToolDef{
		{
			Name:        "add",
			Description: "adds numbers",
			Handler: func(args map[string]any) (any, error) {
				a, _ := args["a"].(float64)
				b, _ := args["b"].(float64)
				return a + b, nil
			},
		},
	})

	// Open SSE stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		// Drain the body to keep connection alive until context cancelled
		buf := make([]byte, 4096)
		for {
			_, err := resp.Body.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Wait for endpoint to be ready
	endpoint, ok := m.WaitForEndpoint(2 * time.Second)
	require.True(t, ok, "endpoint not received")
	assert.Equal(t, m.EndpointURL, endpoint)

	// POST a tool call to the endpoint
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "add",
			"arguments": map[string]any{"a": 2, "b": 3},
		},
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(m.EndpointURL, "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	reqs := m.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "tools/call", reqs[0].Method)
}

func TestSSEMock_DisconnectCleansUp(t *testing.T) {
	m := NewSSEMock(t, nil)

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		buf := make([]byte, 256)
		for {
			_, err := resp.Body.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Wait for connection
	for i := 0; i < 50; i++ {
		if m.IsConnected() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.True(t, m.IsConnected(), "client never connected")

	// Cancel context → disconnect
	cancel()

	// Wait for cleanup
	for i := 0; i < 50; i++ {
		if !m.IsConnected() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.False(t, m.IsConnected(), "client still connected after cancel")
}

func TestStreamableHTTPMock_HeadersRecorded(t *testing.T) {
	m := NewStreamableHTTPMock(t, nil)

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, m.URL, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Auth", "token123")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	reqs := m.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "token123", strings.Join(reqs[0].Headers.Values("X-Custom-Auth"), ", "))
}

func TestSSEMock_GoroutineCleanup(t *testing.T) {
	baseline := runtime.NumGoroutine()
	m := NewSSEMock(t, nil)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	req.Header.Set("Accept", "text/event-stream")

	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		buf := make([]byte, 256)
		for {
			_, err := resp.Body.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Wait for connection
	for i := 0; i < 50; i++ {
		if m.IsConnected() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.True(t, m.IsConnected())

	cancel()
	for i := 0; i < 50; i++ {
		if !m.IsConnected() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Give runtime time to reap goroutines
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+3, "possible goroutine leak: baseline=%d after=%d", baseline, after)
}

func TestStreamableHTTPMock_ConcurrentCalls(t *testing.T) {
	m := NewStreamableHTTPMock(t, []ToolDef{
		{
			Name: "echo",
			Handler: func(args map[string]any) (any, error) {
				return args["text"], nil
			},
		},
	})

	const n = 10
	for i := 0; i < n; i++ {
		go func(idx int) {
			body := map[string]any{
				"jsonrpc": "2.0",
				"id":      idx,
				"method":  "tools/call",
				"params": map[string]any{
					"name":      "echo",
					"arguments": map[string]any{"text": fmt.Sprintf("msg-%d", idx)},
				},
			}
			b, _ := json.Marshal(body)
			resp, err := http.Post(m.URL, "application/json", bytes.NewReader(b))
			if err != nil {
				return
			}
			resp.Body.Close()
		}(i)
	}

	// Wait for all to complete
	time.Sleep(500 * time.Millisecond)
	reqs := m.Requests()
	assert.Len(t, reqs, n)
}
