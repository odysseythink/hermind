package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func echoBinPath(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("MCP_ECHO_BIN")
	require.NotEmpty(t, bin, "MCP_ECHO_BIN not set")
	return bin
}

func bootEcho(t *testing.T, e *mcpTestEnv, opts ...func(*mcp.ServerConfig)) {
	t.Helper()
	srv := mcp.ServerConfig{Name: "echo", Command: echoBinPath(t)}
	for _, o := range opts {
		o(&srv)
	}
	e.writeConfig(t, []mcp.ServerConfig{srv})
	require.NoError(t, e.Hyp.Boot(reqContext(t)))
}

// ── Body & parsing errors ──

func TestMCPHandler_CallTool_BodyTooLarge(t *testing.T) {
	e := newMCPTestEnv(t)
	// Build a valid JSON payload that exceeds 10 MiB.
	large := strings.Repeat("x", 11<<20)
	payload, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": large}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "BODY_TOO_LARGE", resp["errorCode"])
}

func TestMCPHandler_CallTool_MalformedJSON(t *testing.T) {
	e := newMCPTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "INVALID_BODY", resp["errorCode"])
}

// ── Schema / lookup errors ──

func TestMCPHandler_CallTool_ToolNotOnServer(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/nope/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "TOOL_NOT_FOUND", resp["errorCode"])
}

func TestMCPHandler_CallTool_SchemaMismatch(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{}}) // missing required "text"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "ARGS_SCHEMA_MISMATCH", resp["errorCode"])
	assert.Contains(t, resp, "details")
}

// ── Timeout ──

func TestMCPHandler_CallTool_TimeoutQueryOutOfRange(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "hi"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call?timeout=999s", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "INVALID_PARAMS", resp["errorCode"])
}

func TestMCPHandler_CallTool_TimeoutExceeded(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "slow", "delay_ms": 5000}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/slow_echo/call?timeout=1s", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	e.Router.ServeHTTP(w, req)
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 3*time.Second, "should time out quickly")
	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "CALL_TIMEOUT", resp["errorCode"])
}

// ── Concurrency limit ──

func TestMCPHandler_CallTool_ConcurrencyLimit(t *testing.T) {
	e := newMCPTestEnv(t)
	maxOne := 1
	bootEcho(t, e, func(s *mcp.ServerConfig) {
		s.Hermind = &mcp.HermindOptions{MaxConcurrency: &maxOne}
	})

	var wg sync.WaitGroup
	var codes []int
	var mu sync.Mutex

	// Fire two concurrent slow_echo calls.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "block", "delay_ms": 3000}})
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/slow_echo/call?timeout=5s", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			e.Router.ServeHTTP(w, req)
			mu.Lock()
			codes = append(codes, w.Code)
			mu.Unlock()
		}()
	}
	wg.Wait()

	// One should succeed (200) or time out (504); the other should be rejected (429).
	has429 := false
	has200Or504 := false
	for _, c := range codes {
		switch c {
		case http.StatusTooManyRequests:
			has429 = true
		case http.StatusOK, http.StatusGatewayTimeout:
			has200Or504 = true
		}
	}
	assert.True(t, has429, "expected one call to be rejected with 429")
	assert.True(t, has200Or504, "expected one call to succeed or time out")
}

// ── Transport error ──

func TestMCPHandler_CallTool_TransportError(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)

	// Find the running process and kill it so the next CallTool gets a transport error.
	servers, err := e.Hyp.Servers(context.Background())
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.NotNil(t, servers[0].Process)
	pid := servers[0].Process.PID
	require.Greater(t, pid, 0)

	proc, err := os.FindProcess(pid)
	require.NoError(t, err)
	require.NoError(t, proc.Kill())
	// Wait a moment for the process to actually die.
	time.Sleep(100 * time.Millisecond)

	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "hi"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)

	// Killing the child may result in either a transport error (502) or a timeout
	// if the transport hangs waiting for a dead connection. Accept both.
	assert.Contains(t, []int{http.StatusBadGateway, http.StatusGatewayTimeout}, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	// If it's a timeout, errorCode is CALL_TIMEOUT; otherwise TRANSPORT_ERROR.
	code, _ := resp["errorCode"].(string)
	assert.True(t, code == "TRANSPORT_ERROR" || code == "CALL_TIMEOUT", "unexpected errorCode: %s", code)
}

// ── Audit log ──

func TestMCPHandler_CallTool_AuditLog_Success(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "audit-test"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Audit log is async; poll briefly.
	var logs []models.EventLog
	for i := 0; i < 20; i++ {
		e.DB.Where("event = ?", "mcp.call.success").Find(&logs)
		if len(logs) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Len(t, logs, 1)
	assert.Equal(t, "mcp.call.success", logs[0].Event)
	assert.NotNil(t, logs[0].Metadata)
	metaStr := *logs[0].Metadata
	assert.Contains(t, metaStr, "echo")
	assert.Contains(t, metaStr, "duration_ms")
}

func TestMCPHandler_CallTool_AuditLog_Failure(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{}}) // schema mismatch
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	// Schema-mismatch failures are rejected before the transport call, so no
	// mcp.call.failed audit event is written for ARGS_SCHEMA_MISMATCH (the
	// plan only logs failed for CALL_TIMEOUT and TRANSPORT_ERROR).
	// This test asserts the negative: no failed audit row for schema mismatch.
	var logs []models.EventLog
	e.DB.Where("event = ?", "mcp.call.failed").Find(&logs)
	assert.Empty(t, logs)
}

func TestMCPHandler_CallTool_AuditLog_BestEffort(t *testing.T) {
	// A failing event-log service should not block the tool call.
	e := newMCPTestEnv(t)
	// Close the underlying DB to make LogEvent fail.
	sqlDB, err := e.DB.DB()
	require.NoError(t, err)
	sqlDB.Close()

	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "best-effort"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
}

// ── Happy path ──

func TestMCPHandler_CallTool_EchoTool_E2E_WithTimeoutQuery(t *testing.T) {
	e := newMCPTestEnv(t)
	bootEcho(t, e)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "hello-timeout"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call?timeout=5s", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.NotNil(t, resp["result"])
}
