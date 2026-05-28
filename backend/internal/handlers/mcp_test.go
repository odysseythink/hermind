package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mcpTestEnv struct {
	Router      *gin.Engine
	Storage     string
	Cfg         *config.Config
	Hyp         *mcp.Hypervisor
	Svc         *services.MCPService
	AuthSvc     *services.AuthService
	EventLogSvc *services.EventLogService
	DB          *gorm.DB
}

func newMCPTestEnv(t *testing.T) *mcpTestEnv {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{StorageDir: tmp} // AuthToken == "" → IsAuthEnabled() == false → bypass auth
	// Use a unique in-memory DB name per test so parallel tests don't share tables.
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", tmp)), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, services.AutoMigrate(db))

	authSvc := services.NewAuthService(db, cfg, nil)
	eventLogSvc := services.NewEventLogService(db)
	hyp := mcp.NewHypervisorForTesting(cfg)
	svc := services.NewMCPService(hyp)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	RegisterMCPRoutes(api, authSvc, svc, eventLogSvc, cfg)
	return &mcpTestEnv{Router: r, Storage: tmp, Cfg: cfg, Hyp: hyp, Svc: svc, AuthSvc: authSvc, EventLogSvc: eventLogSvc, DB: db}
}

func (e *mcpTestEnv) writeRawConfig(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(e.Storage, "plugins", "hermind_mcp_servers.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0644))
}

func (e *mcpTestEnv) writeConfig(t *testing.T, servers []mcp.ServerConfig) {
	t.Helper()
	path := filepath.Join(e.Storage, "plugins", "hermind_mcp_servers.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	c := mcp.NewConfig(e.Storage)
	require.NoError(t, c.Write(servers))
}

func TestMCPHandler_ListServers_Empty(t *testing.T) {
	e := newMCPTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/mcp-servers/list", nil)
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Empty(t, resp["servers"])
}

func TestMCPHandler_ListServers_OneServer(t *testing.T) {
	e := newMCPTestEnv(t)
	e.writeRawConfig(t, `{"mcpServers":{"echo":{"command":"node","args":["echo-server.js"]}}}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/mcp-servers/list", nil)
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	servers, ok := resp["servers"].([]any)
	require.True(t, ok)
	require.Len(t, servers, 1)
	srv := servers[0].(map[string]any)
	assert.Equal(t, "echo", srv["name"])
	assert.Equal(t, false, srv["running"])
	assert.NotNil(t, srv["error"])
	assert.Equal(t, "MCP transport not implemented", srv["error"])
	assert.Empty(t, srv["tools"])
	assert.Nil(t, srv["process"])
}

func TestMCPHandler_ForceReload_Success(t *testing.T) {
	e := newMCPTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/mcp-servers/force-reload", nil)
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Nil(t, resp["error"])
	assert.Empty(t, resp["servers"])
}

func TestMCPHandler_ToggleServer_NotFound(t *testing.T) {
	e := newMCPTestEnv(t)
	body, _ := json.Marshal(map[string]string{"name": "echo"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Contains(t, resp["error"], "not found")
}

func TestMCPHandler_ToggleServer_MissingNameBody(t *testing.T) {
	e := newMCPTestEnv(t)
	body, _ := json.Marshal(map[string]string{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Contains(t, resp["error"], "name")
}

func TestMCPHandler_DeleteServer_Found(t *testing.T) {
	e := newMCPTestEnv(t)
	e.writeRawConfig(t, `{"mcpServers":{"echo":{"command":"node"}}}`)
	body, _ := json.Marshal(map[string]string{"name": "echo"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Nil(t, resp["error"])

	servers, err := e.Hyp.Servers(req.Context())
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestMCPHandler_DeleteServer_NotFound(t *testing.T) {
	e := newMCPTestEnv(t)
	body, _ := json.Marshal(map[string]string{"name": "echo"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Contains(t, resp["error"], "not found")
}

func TestMCPHandler_ToggleTool_Suppress(t *testing.T) {
	e := newMCPTestEnv(t)
	e.writeRawConfig(t, `{"mcpServers":{"echo":{"command":"node"}}}`)
	body, _ := json.Marshal(map[string]any{"serverName": "echo", "toolName": "danger", "enabled": false})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/toggle-tool", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Nil(t, resp["error"])
	suppressed, ok := resp["suppressedTools"].([]any)
	require.True(t, ok)
	assert.Len(t, suppressed, 1)
	assert.Equal(t, "danger", suppressed[0])
}

func TestMCPHandler_ToggleTool_Unsuppress(t *testing.T) {
	e := newMCPTestEnv(t)
	e.writeRawConfig(t, `{"mcpServers":{"echo":{"command":"node","hermind":{"suppressedTools":["danger"]}}}}`)
	body, _ := json.Marshal(map[string]any{"serverName": "echo", "toolName": "danger", "enabled": true})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/toggle-tool", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Nil(t, resp["error"])
	suppressed, ok := resp["suppressedTools"].([]any)
	require.True(t, ok)
	assert.Empty(t, suppressed)
}

func TestMCPHandler_ToggleTool_ServerNotFound(t *testing.T) {
	e := newMCPTestEnv(t)
	body, _ := json.Marshal(map[string]any{"serverName": "missing", "toolName": "danger", "enabled": false})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/toggle-tool", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Contains(t, resp["error"], "not found")
	suppressed, ok := resp["suppressedTools"].([]any)
	require.True(t, ok)
	assert.Empty(t, suppressed)
}

func TestMCPHandler_CallTool_ServerNotRunning(t *testing.T) {
	e := newMCPTestEnv(t)
	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/do/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Nil(t, resp["result"])
	assert.Equal(t, "SERVER_NOT_FOUND", resp["errorCode"])
}

func TestMCPHandler_CallTool_BadJSON(t *testing.T) {
	e := newMCPTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/do/call", bytes.NewReader([]byte(`{bad`)))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	// ShouldBindJSON fails to parse malformed JSON; handler returns 400 INVALID_BODY
	assert.Equal(t, 400, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Nil(t, resp["result"])
	assert.Equal(t, "INVALID_BODY", resp["errorCode"])
}

func echoBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("MCP_ECHO_BIN")
	if bin != "" {
		return bin
	}
	// Build into a system temp dir (not t.TempDir) so the test's temp
	// cleanup doesn't try to delete a locked binary on Windows.
	tmp, err := os.MkdirTemp("", "mcp-echo-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmp) })
	bin = filepath.Join(tmp, "echo-mcp")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	echoDir, err := filepath.Abs("../mcp/testdata/echo-mcp")
	require.NoError(t, err)
	cmd := exec.Command("go", "build", "-C", echoDir, "-o", bin, ".")
	require.NoError(t, cmd.Run())
	return bin
}

func TestMCPHandler_ToggleServer_StartsRealProcess(t *testing.T) {
	e := newMCPTestEnv(t)
	e.writeConfig(t, []mcp.ServerConfig{{Name: "echo", Command: echoBin(t)}})

	body, _ := json.Marshal(map[string]string{"name": "echo"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Nil(t, resp["error"])

	// Verify the server is running
	servers, err := e.Hyp.Servers(req.Context())
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.True(t, servers[0].Running)
	assert.NotNil(t, servers[0].Process)
	assert.Greater(t, servers[0].Process.PID, 0)
}

func TestMCPHandler_CallTool_EchoTool_E2E(t *testing.T) {
	e := newMCPTestEnv(t)
	e.writeConfig(t, []mcp.ServerConfig{{Name: "echo", Command: echoBin(t)}})
	require.NoError(t, e.Hyp.Boot(reqContext(t)))

	body, _ := json.Marshal(map[string]any{"arguments": map[string]any{"text": "hello-from-handler"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp/echo/tools/echo/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.NotNil(t, resp["result"])
}

func reqContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

func TestMCPHandler_DeleteServer_KillsRunningProcess(t *testing.T) {
	e := newMCPTestEnv(t)
	e.writeConfig(t, []mcp.ServerConfig{{Name: "echo", Command: echoBin(t)}})
	require.NoError(t, e.Hyp.Boot(reqContext(t)))

	servers, err := e.Hyp.Servers(context.Background())
	require.NoError(t, err)
	require.Len(t, servers, 1)
	pid := servers[0].Process.PID
	require.Greater(t, pid, 0)

	body, _ := json.Marshal(map[string]string{"name": "echo"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/mcp-servers/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.Router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Nil(t, resp["error"])

	// Verify process is gone (POSIX only; skipped on Windows)
	if runtime.GOOS != "windows" {
		proc, _ := os.FindProcess(pid)
		if proc != nil {
			assert.NotNil(t, proc.Signal(syscall.Signal(0)))
		}
	}
}
