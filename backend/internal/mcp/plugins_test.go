package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHypervisor_ActiveServers_Empty(t *testing.T) {
	h, _ := newTestHypervisor(t)
	servers := h.ActiveServers()
	assert.NotNil(t, servers)
	assert.Empty(t, servers)
	assert.Equal(t, []string{}, servers)
}

func TestHypervisor_ActiveServers_OneRunning(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers := h.ActiveServers()
	require.Len(t, servers, 1)
	assert.Equal(t, "@@mcp_echo", servers[0])
}

func TestHypervisor_ActiveServers_TwoRunning_Sorted(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{
		{Name: "zebra", Command: echoBinPath(t)},
		{Name: "alpha", Command: echoBinPath(t)},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers := h.ActiveServers()
	require.Len(t, servers, 2)
	assert.Equal(t, []string{"@@mcp_alpha", "@@mcp_zebra"}, servers)
}

func TestHypervisor_ActiveServers_ExcludesFailedBoot(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{
		{Name: "good", Command: echoBinPath(t)},
		{Name: "bad", Command: "/nonexistent/binary"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers := h.ActiveServers()
	require.Len(t, servers, 1)
	assert.Equal(t, "@@mcp_good", servers[0])
}

func TestHypervisor_ActiveServers_ExcludesPrunedServer(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	writeConfig(t, tmp, []ServerConfig{
		{Name: "alpha", Command: echoBinPath(t)},
		{Name: "beta", Command: echoBinPath(t)},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ok, err := h.ToggleServer(ctx, "alpha")
	require.NoError(t, err)
	assert.True(t, ok)

	servers := h.ActiveServers()
	require.Len(t, servers, 1)
	assert.Equal(t, "@@mcp_beta", servers[0])
}

func TestHypervisor_ActiveServers_AutoStartFalseExcluded(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp, func(s *ServerConfig) {
		autoStart := false
		s.AnythingLLM = &AnythingLLMOptions{AutoStart: &autoStart}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	servers := h.ActiveServers()
	assert.Empty(t, servers)
}

// ── Task 2: ToolsAsPlugins metadata ──

func TestHypervisor_ToolsAsPlugins_ServerNotFound(t *testing.T) {
	h, _ := newTestHypervisor(t)
	plugins, err := h.ToolsAsPlugins("ghost")
	assert.Nil(t, plugins)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_ToolsAsPlugins_PrunedServer(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ok, err := h.ToggleServer(ctx, "echo")
	require.NoError(t, err)
	assert.True(t, ok)

	plugins, err := h.ToolsAsPlugins("echo")
	assert.Nil(t, plugins)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_ToolsAsPlugins_ReturnsAllTools(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)
	require.Len(t, plugins, 3)

	byName := make(map[string]ToolPlugin)
	for _, p := range plugins {
		byName[p.ToolName] = p
	}

	for _, name := range []string{"echo", "add", "slow_echo"} {
		p, ok := byName[name]
		require.True(t, ok, "missing tool %s", name)
		assert.Equal(t, "echo", p.ServerName)
		assert.Equal(t, name, p.ToolName)
		assert.Equal(t, "echo-"+name, p.QualifiedName)
		assert.NotEmpty(t, p.Description)
		assert.NotNil(t, p.Call, "Call closure must be present")
	}
}

func TestHypervisor_ToolsAsPlugins_FiltersSuppressed(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp, func(s *ServerConfig) {
		s.AnythingLLM = &AnythingLLMOptions{SuppressedTools: []string{"add"}}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)
	require.Len(t, plugins, 2)

	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.ToolName
	}
	assert.ElementsMatch(t, []string{"echo", "slow_echo"}, names)
}

func TestHypervisor_ToolsAsPlugins_AllToolsSuppressed_EmptyNotNil(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp, func(s *ServerConfig) {
		s.AnythingLLM = &AnythingLLMOptions{SuppressedTools: []string{"echo", "add", "slow_echo"}}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)
	require.NotNil(t, plugins)
	assert.Empty(t, plugins)
	assert.Equal(t, []ToolPlugin{}, plugins)
}

func TestHypervisor_ToolsAsPlugins_CarriesInputSchema(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)

	var echoPlugin *ToolPlugin
	for i := range plugins {
		if plugins[i].ToolName == "echo" {
			echoPlugin = &plugins[i]
			break
		}
	}
	require.NotNil(t, echoPlugin)

	ts, err := h.GetToolSchema("echo", "echo")
	require.NoError(t, err)
	assert.Equal(t, string(ts.InputSchema), string(echoPlugin.InputSchema))
}

func TestHypervisor_ToolsAsPlugins_NoInputSchemaToolHasNilField(t *testing.T) {
	h, _ := newTestHypervisor(t)
	require.NoError(t, h.Boot(context.Background()))
	h.mu.Lock()
	h.mcps["no-schema"] = &activeClient{
		transport: nil,
		process:   nil,
		tools: []ToolSchema{
			{Name: "noop", Description: "does nothing"},
		},
		schemaByName: map[string]json.RawMessage{},
	}
	h.mu.Unlock()

	plugins, err := h.ToolsAsPlugins("no-schema")
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "noop", plugins[0].ToolName)
	assert.True(t, len(plugins[0].InputSchema) == 0 || plugins[0].InputSchema == nil)
}

// ── Task 3: ToolPlugin.Call closure ──

func defaultArgsFor(toolName string) map[string]any {
	switch toolName {
	case "echo":
		return map[string]any{"text": "hi"}
	case "add":
		return map[string]any{"a": 2, "b": 3}
	case "slow_echo":
		return map[string]any{"text": "slow", "delay_ms": 5000}
	default:
		return map[string]any{}
	}
}

func TestToolPlugin_Call_RoundtripEcho(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)

	var echoPlugin *ToolPlugin
	for i := range plugins {
		if plugins[i].ToolName == "echo" {
			echoPlugin = &plugins[i]
			break
		}
	}
	require.NotNil(t, echoPlugin)

	result, err := echoPlugin.Call(ctx, map[string]any{"text": "hello-plugin"})
	require.NoError(t, err)
	ctr, ok := result.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, ctr.Content, 1)
	tc, ok := mcp.AsTextContent(ctr.Content[0])
	require.True(t, ok)
	assert.Equal(t, "hello-plugin", tc.Text)
}

func TestToolPlugin_Call_RoundtripAdd(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)

	var addPlugin *ToolPlugin
	for i := range plugins {
		if plugins[i].ToolName == "add" {
			addPlugin = &plugins[i]
			break
		}
	}
	require.NotNil(t, addPlugin)

	result, err := addPlugin.Call(ctx, map[string]any{"a": 2, "b": 3})
	require.NoError(t, err)
	ctr, ok := result.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, ctr.Content, 1)
	tc, ok := mcp.AsTextContent(ctr.Content[0])
	require.True(t, ok)
	assert.Contains(t, tc.Text, "sum=5")
}

func TestToolPlugin_Call_ServerPrunedAfterPluginObtained(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(plugins), 1)
	p := plugins[0]

	ok, err := h.ToggleServer(ctx, "echo")
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = p.Call(ctx, defaultArgsFor(p.ToolName))
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestToolPlugin_Call_NoLoopVariableCaptureBug(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(plugins), 2)

	// Snapshot ALL plugins first, then call each — ensures any closure-capture
	// bug surfaces as "all plugins called the last tool".
	results := make(map[string]any)
	for _, p := range plugins {
		r, callErr := p.Call(ctx, defaultArgsFor(p.ToolName))
		require.NoError(t, callErr, "tool %s failed", p.ToolName)
		results[p.QualifiedName] = r
	}
	// Each plugin must have returned a distinct result keyed by qualifiedName.
	assert.Len(t, results, len(plugins))

	// Additional sanity: echo and add produce different text results.
	if echoResult, ok := results["echo-echo"]; ok {
		ctr, ok := echoResult.(*mcp.CallToolResult)
		require.True(t, ok)
		tc, _ := mcp.AsTextContent(ctr.Content[0])
		assert.Equal(t, "hi", tc.Text)
	}
	if addResult, ok := results["echo-add"]; ok {
		ctr, ok := addResult.(*mcp.CallToolResult)
		require.True(t, ok)
		tc, _ := mcp.AsTextContent(ctr.Content[0])
		assert.Contains(t, tc.Text, "sum=5")
	}
}

func TestToolPlugin_Call_ContextCanceled(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(plugins), 1)
	p := plugins[0]

	canceledCtx, cancelCtx := context.WithCancel(context.Background())
	cancelCtx()

	_, err = p.Call(canceledCtx, defaultArgsFor(p.ToolName))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestToolPlugin_Call_ContextDeadline(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	plugins, err := h.ToolsAsPlugins("echo")
	require.NoError(t, err)

	var slowPlugin *ToolPlugin
	for i := range plugins {
		if plugins[i].ToolName == "slow_echo" {
			slowPlugin = &plugins[i]
			break
		}
	}
	require.NotNil(t, slowPlugin, "slow_echo tool required for deadline test")

	slowCtx, cancelSlow := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelSlow()

	_, err = slowPlugin.Call(slowCtx, map[string]any{"text": "too-slow", "delay_ms": 5000})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
