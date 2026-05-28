package mcp

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/mcp/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHypervisor(t *testing.T) (*Hypervisor, string) {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{StorageDir: tmp}
	h := NewHypervisorForTesting(cfg)
	return h, tmp
}

func writeRawConfig(t *testing.T, storage, body string) {
	t.Helper()
	path := filepath.Join(storage, "plugins", "anythingllm_mcp_servers.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0644))
}

func writeConfig(t *testing.T, storage string, servers []ServerConfig) {
	t.Helper()
	path := filepath.Join(storage, "plugins", "anythingllm_mcp_servers.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	c := NewConfig(storage)
	require.NoError(t, c.Write(servers))
}

func seedEchoConfig(t *testing.T, storage string, opts ...func(*ServerConfig)) {
	t.Helper()
	srv := ServerConfig{Name: "echo", Command: echoBinPath(t)}
	for _, o := range opts {
		o(&srv)
	}
	writeConfig(t, storage, []ServerConfig{srv})
}

func echoBinPath(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("MCP_ECHO_BIN")
	require.NotEmpty(t, bin, "MCP_ECHO_BIN not set")
	return bin
}

func TestHypervisor_Servers_EmptyConfig(t *testing.T) {
	h, _ := newTestHypervisor(t)
	servers, err := h.Servers(context.Background())
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestHypervisor_Servers_OneServer(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeRawConfig(t, tmp, `{"mcpServers":{"echo":{"command":"node","args":["echo-server.js"]}}}`)
	servers, err := h.Servers(context.Background())
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "echo", servers[0].Name)
	assert.False(t, servers[0].Running)
	assert.NotNil(t, servers[0].Error)
	assert.Equal(t, "MCP transport not implemented", *servers[0].Error)
	assert.Empty(t, servers[0].Tools)
	assert.Nil(t, servers[0].Process)
}

func TestHypervisor_Servers_TwoServers_OrderStable(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeRawConfig(t, tmp, `{"mcpServers":{"a":{"command":"node"},"b":{"url":"http://x"}}}`)
	servers, err := h.Servers(context.Background())
	require.NoError(t, err)
	require.Len(t, servers, 2)
	names := make(map[string]bool)
	for _, s := range servers {
		names[s.Name] = true
	}
	assert.True(t, names["a"])
	assert.True(t, names["b"])
}

func TestHypervisor_Reload_SameAsServers(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeRawConfig(t, tmp, `{"mcpServers":{"echo":{"command":"node"}}}`)
	reloaded, err := h.Reload(context.Background())
	require.NoError(t, err)
	servers, err := h.Servers(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(servers), len(reloaded))
}

func TestHypervisor_ToggleServer_NotFound(t *testing.T) {
	h, _ := newTestHypervisor(t)
	ok, err := h.ToggleServer(context.Background(), "echo")
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_DeleteServer_Found(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeRawConfig(t, tmp, `{"mcpServers":{"echo":{"command":"node"}}}`)
	ok, err := h.DeleteServer(context.Background(), "echo")
	require.NoError(t, err)
	assert.True(t, ok)
	servers, err := h.Servers(context.Background())
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestHypervisor_DeleteServer_NotFound(t *testing.T) {
	h, _ := newTestHypervisor(t)
	ok, err := h.DeleteServer(context.Background(), "echo")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrServerNotFound)
	assert.False(t, ok)
}

func TestHypervisor_ToggleTool_Suppress(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeRawConfig(t, tmp, `{"mcpServers":{"echo":{"command":"node"}}}`)
	out, err := h.ToggleTool(context.Background(), "echo", "danger", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"danger"}, out)
}

func TestHypervisor_ToggleTool_Unsuppress(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeRawConfig(t, tmp, `{"mcpServers":{"echo":{"command":"node","anythingllm":{"suppressedTools":["danger"]}}}}`)
	out, err := h.ToggleTool(context.Background(), "echo", "danger", true)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestHypervisor_ToggleTool_ServerNotFound(t *testing.T) {
	h, _ := newTestHypervisor(t)
	out, err := h.ToggleTool(context.Background(), "missing", "tool", false)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_CallTool_ServerNotRunning(t *testing.T) {
	h, _ := newTestHypervisor(t)
	result, err := h.CallTool(context.Background(), "echo", "do", nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_Boot_Noop(t *testing.T) {
	h, _ := newTestHypervisor(t)
	err := h.Boot(context.Background())
	require.NoError(t, err)
}

func TestHypervisor_PruneAll_Noop(t *testing.T) {
	h, _ := newTestHypervisor(t)
	err := h.PruneAll()
	require.NoError(t, err)
}

func TestHypervisor_ParseServerType(t *testing.T) {
	tests := []struct {
		name     string
		input    *ServerConfig
		expected string
	}{
		{"sse", &ServerConfig{Type: "sse"}, "sse"},
		{"streamable", &ServerConfig{Type: "streamable"}, "http"},
		{"http", &ServerConfig{Type: "http"}, "http"},
		{"stdio by command", &ServerConfig{Command: "node"}, "stdio"},
		{"http by url fallback", &ServerConfig{URL: "http://x"}, "http"},
		{"empty", &ServerConfig{}, "sse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseServerType(tt.input))
		})
	}
}

// TestHypervisor_ValidateServerDefinition_StdioArgsMustBeArray is skipped
// because Args is []string in Go, so the runtime type check that Node
// performs (ensuring args is an array) is enforced by the type system.
func TestHypervisor_ValidateServerDefinition_StdioArgsMustBeArray(t *testing.T) {
	t.Skip("Go type system enforces []string for Args — no runtime check needed")
}

func TestHypervisor_ValidateServerDefinition_HTTPUrlRequired(t *testing.T) {
	err := validateServerDefinition("x", &ServerConfig{Type: "http"}, "http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required")
}

func TestHypervisor_ValidateServerDefinition_HTTPUrlMalformed(t *testing.T) {
	err := validateServerDefinition("x", &ServerConfig{Type: "http", URL: "://invalid"}, "http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

// ── PR-B integration tests ──

func TestHypervisor_Boot_StartsAutoStartServer(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	h.mu.RLock()
	defer h.mu.RUnlock()
	require.Len(t, h.mcps, 1)
	assert.Equal(t, "success", h.results["echo"].Status)
}

func TestHypervisor_Boot_SkipsAutoStartFalse(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp, func(s *ServerConfig) { autoStart := false; s.AnythingLLM = &AnythingLLMOptions{AutoStart: &autoStart} })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.Empty(t, h.mcps)
	assert.Equal(t, "failed", h.results["echo"].Status)
	assert.Contains(t, h.results["echo"].Message, "autoStart")
}

func TestHypervisor_Boot_FailureContinuesOthers(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{Name: "good", Command: echoBinPath(t)}, {Name: "bad", Command: "/nonexistent/binary"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.Len(t, h.mcps, 1)
	assert.Equal(t, "success", h.results["good"].Status)
	assert.Equal(t, "failed", h.results["bad"].Status)
}

func TestHypervisor_Boot_30sTimeoutOnHangingServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command not available on Windows")
	}
	old := connectionTimeoutForTest
	connectionTimeoutForTest = 2 * time.Second
	t.Cleanup(func() { connectionTimeoutForTest = old })

	h, tmp := newTestHypervisor(t)
	writeRawConfig(t, tmp, `{"mcpServers":{"hang":{"command":"sleep","args":["60"]}}}`)

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second, "should time out quickly with test override")
	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.Equal(t, "failed", h.results["hang"].Status)
}

func TestHypervisor_Servers_AfterBoot(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "echo", servers[0].Name)
	assert.True(t, servers[0].Running)
	assert.NotNil(t, servers[0].Process)
	assert.Greater(t, servers[0].Process.PID, 0)
	assert.Nil(t, servers[0].Error)

	require.Len(t, servers[0].Tools, 3)
	names := make([]string, len(servers[0].Tools))
	for i, tl := range servers[0].Tools {
		names[i] = tl.Name
	}
	assert.ElementsMatch(t, []string{"echo", "add", "slow_echo"}, names)
}

func TestHypervisor_Servers_FiltersSuppressedTools(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp, func(s *ServerConfig) { s.AnythingLLM = &AnythingLLMOptions{SuppressedTools: []string{"add"}} })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Len(t, servers[0].Tools, 2)
	names := make([]string, len(servers[0].Tools))
	for i, tl := range servers[0].Tools {
		names[i] = tl.Name
	}
	assert.ElementsMatch(t, []string{"echo", "slow_echo"}, names)
}

func TestHypervisor_ToggleServer_Off(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ok, err := h.ToggleServer(ctx, "echo")
	require.NoError(t, err)
	assert.True(t, ok)

	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.NotContains(t, h.mcps, "echo")
}

func TestHypervisor_ToggleServer_On(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp, func(s *ServerConfig) { autoStart := false; s.AnythingLLM = &AnythingLLMOptions{AutoStart: &autoStart} })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ok, err := h.ToggleServer(ctx, "echo")
	require.NoError(t, err)
	assert.True(t, ok)

	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.Contains(t, h.mcps, "echo")
}

func TestHypervisor_ToggleServer_UnknownName(t *testing.T) {
	h, _ := newTestHypervisor(t)
	ok, err := h.ToggleServer(context.Background(), "ghost")
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_DeleteServer_KillsAndRemoves(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ok, err := h.DeleteServer(ctx, "echo")
	require.NoError(t, err)
	assert.True(t, ok)

	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.NotContains(t, h.mcps, "echo")

	servers, err := h.file.Load()
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestHypervisor_CallTool_Echo(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	res, err := h.CallTool(ctx, "echo", "echo", map[string]any{"text": "hello"})
	require.NoError(t, err)
	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	tc, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Equal(t, "hello", tc.Text)
}

func TestHypervisor_CallTool_UnknownServer(t *testing.T) {
	h, _ := newTestHypervisor(t)
	_, err := h.CallTool(context.Background(), "ghost", "tool", nil)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_PruneAll_KillsEverything(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	require.NoError(t, h.PruneAll())

	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.Empty(t, h.mcps)
}

func TestHypervisor_Reload_RestartsAfterPrune(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))
	require.NoError(t, h.PruneAll())

	// Boot should repopulate mcps because len(mcps) == 0 after prune.
	require.NoError(t, h.Boot(ctx))

	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.Len(t, h.mcps, 1)
}

// ── PR-C integration tests ──

func TestHypervisor_Boot_HTTPServer(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{Name: "echo", Description: "echo"},
		{Name: "add", Description: "add"},
	})
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{Name: "http-mock", URL: m.URL, Type: "http"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "http-mock", servers[0].Name)
	assert.True(t, servers[0].Running)
	assert.Nil(t, servers[0].Process)
	assert.Nil(t, servers[0].Error)
	require.Len(t, servers[0].Tools, 2)
}

func TestHypervisor_Boot_SSEServer(t *testing.T) {
	m := testutil.NewSSEMock(t, []testutil.ToolDef{
		{Name: "echo", Description: "echo"},
		{Name: "add", Description: "add"},
	})
	h, tmp := newTestHypervisor(t)
	defer h.PruneAll()
	writeConfig(t, tmp, []ServerConfig{{Name: "sse-mock", URL: m.URL, Type: "sse"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "sse-mock", servers[0].Name)
	assert.True(t, servers[0].Running)
	assert.Nil(t, servers[0].Process)
	assert.Nil(t, servers[0].Error)
	require.Len(t, servers[0].Tools, 2)
}

func TestHypervisor_CallTool_HTTPServer_E2E(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{
			Name: "echo",
			Handler: func(args map[string]any) (any, error) {
				return args["text"], nil
			},
		},
	})
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{Name: "http-mock", URL: m.URL, Type: "http"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	res, err := h.CallTool(ctx, "http-mock", "echo", map[string]any{"text": "hello-http"})
	require.NoError(t, err)
	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	tc, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Equal(t, "hello-http", tc.Text)
}

func TestHypervisor_CallTool_SSEServer_E2E(t *testing.T) {
	m := testutil.NewSSEMock(t, []testutil.ToolDef{
		{
			Name: "echo",
			Handler: func(args map[string]any) (any, error) {
				return args["text"], nil
			},
		},
	})
	h, tmp := newTestHypervisor(t)
	defer h.PruneAll()
	writeConfig(t, tmp, []ServerConfig{{Name: "sse-mock", URL: m.URL, Type: "sse"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	res, err := h.CallTool(ctx, "sse-mock", "echo", map[string]any{"text": "hello-sse"})
	require.NoError(t, err)
	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, result.Content, 1)
	tc, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Equal(t, "hello-sse", tc.Text)
}

func TestHypervisor_ToggleServer_HTTPServer_Off_Then_On(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{Name: "http-mock", URL: m.URL, Type: "http"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	// Toggle off
	ok, err := h.ToggleServer(ctx, "http-mock")
	require.NoError(t, err)
	assert.True(t, ok)

	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.False(t, servers[0].Running)

	// Toggle on — should re-initialize
	ok, err = h.ToggleServer(ctx, "http-mock")
	require.NoError(t, err)
	assert.True(t, ok)

	reqs := m.Requests()
	// At least 2 initialize requests: first boot + second toggle-on
	initCount := 0
	for _, r := range reqs {
		if r.Method == "initialize" {
			initCount++
		}
	}
	assert.GreaterOrEqual(t, initCount, 2)
}

func TestHypervisor_DeleteServer_HTTPServer(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{Name: "http-mock", URL: m.URL, Type: "http"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ok, err := h.DeleteServer(ctx, "http-mock")
	require.NoError(t, err)
	assert.True(t, ok)

	servers, err := h.file.Load()
	require.NoError(t, err)
	assert.Empty(t, servers)

	// Toggle after delete should fail
	_, err = h.ToggleServer(ctx, "http-mock")
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_Boot_MixedTransports(t *testing.T) {
	httpMock := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{Name: "http-tool", Description: "http"},
	})
	sseMock := testutil.NewSSEMock(t, []testutil.ToolDef{
		{Name: "sse-tool", Description: "sse"},
	})
	h, tmp := newTestHypervisor(t)
	defer h.PruneAll()
	writeConfig(t, tmp, []ServerConfig{
		{Name: "stdio-srv", Command: echoBinPath(t)},
		{Name: "http-srv", URL: httpMock.URL, Type: "http"},
		{Name: "sse-srv", URL: sseMock.URL, Type: "sse"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 3)

	byName := make(map[string]ServerView)
	for _, s := range servers {
		byName[s.Name] = s
	}

	assert.True(t, byName["stdio-srv"].Running)
	assert.NotNil(t, byName["stdio-srv"].Process)
	assert.Len(t, byName["stdio-srv"].Tools, 3)

	assert.True(t, byName["http-srv"].Running)
	assert.Nil(t, byName["http-srv"].Process)
	assert.Len(t, byName["http-srv"].Tools, 1)

	assert.True(t, byName["sse-srv"].Running)
	assert.Nil(t, byName["sse-srv"].Process)
	assert.Len(t, byName["sse-srv"].Tools, 1)
}

func TestHypervisor_Servers_HTTPSuppressionFilters(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, []testutil.ToolDef{
		{Name: "echo", Description: "echo"},
		{Name: "add", Description: "add"},
	})
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{
		Name: "http-mock",
		URL:  m.URL,
		Type: "http",
		AnythingLLM: &AnythingLLMOptions{
			SuppressedTools: []string{"add"},
		},
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Len(t, servers[0].Tools, 1)
	assert.Equal(t, "echo", servers[0].Tools[0].Name)
}

func TestHypervisor_PruneAll_HTTPCleansUp(t *testing.T) {
	m := testutil.NewStreamableHTTPMock(t, nil)
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{Name: "http-mock", URL: m.URL, Type: "http"}})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	// Verify running before prune
	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.True(t, servers[0].Running)

	require.NoError(t, h.PruneAll())

	// After prune, the transport is closed so Ping should be false
	// (Servers reloads config and sees no active mcps)
	servers, err = h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.False(t, servers[0].Running)
}

func TestHypervisor_Boot_HTTPConnectTimeout(t *testing.T) {
	// Use a raw TCP listener that accepts connections but never responds.
	// This avoids httptest.Server.Close() blocking on Windows when
	// handlers never return.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Accept and immediately drop the conn reference — the OS
			// will clean up the half-open socket once the client times
			// out and closes its end.
			_ = conn
		}
	}()

	old := connectionTimeoutForTest
	connectionTimeoutForTest = 2 * time.Second
	t.Cleanup(func() { connectionTimeoutForTest = old })

	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{{Name: "hang", URL: "http://" + ln.Addr().String(), Type: "http"}})

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second, "should time out quickly")
	h.mu.RLock()
	defer h.mu.RUnlock()
	assert.Equal(t, "failed", h.results["hang"].Status)
	assert.Contains(t, h.results["hang"].Message, "deadline")
}
