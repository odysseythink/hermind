package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Ensure_CreatesFileIfMissing(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, c.Ensure())
	data, err := os.ReadFile(c.Path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"mcpServers":{}}`, string(data))
}

func TestConfig_Ensure_NoopIfPresent(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`garbage`), 0644))
	require.NoError(t, c.Ensure())
	data, err := os.ReadFile(c.Path)
	require.NoError(t, err)
	assert.Equal(t, "garbage", string(data))
}

func TestConfig_Load_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(``), 0644))
	servers, err := c.Load()
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestConfig_Load_MalformedJSON(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{bad`), 0644))
	servers, err := c.Load()
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestConfig_Load_StdioServer(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{
		"mcpServers": {
			"echo": {
				"command": "node",
				"args": ["echo-server.js"],
				"env": {"FOO": "bar"}
			}
		}
	}`), 0644))
	servers, err := c.Load()
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "echo", servers[0].Name)
	assert.Equal(t, "node", servers[0].Command)
	assert.Equal(t, []string{"echo-server.js"}, servers[0].Args)
	assert.Equal(t, map[string]string{"FOO": "bar"}, servers[0].Env)
}

func TestConfig_Load_HTTPServer(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{
		"mcpServers": {
			"stream": {
				"url": "http://localhost:3000/sse",
				"type": "streamable",
				"headers": {"Authorization": "Bearer token"}
			}
		}
	}`), 0644))
	servers, err := c.Load()
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "stream", servers[0].Name)
	assert.Equal(t, "http://localhost:3000/sse", servers[0].URL)
	assert.Equal(t, "streamable", servers[0].Type)
	assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, servers[0].Headers)
}

func TestConfig_Write_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	in := []ServerConfig{
		{Name: "a", Command: "cmd"},
		{Name: "b", URL: "http://x"},
	}
	require.NoError(t, c.Write(in))
	out, err := c.Load()
	require.NoError(t, err)
	require.Len(t, out, 2)

	byName := make(map[string]ServerConfig)
	for _, s := range out {
		byName[s.Name] = s
	}
	assert.Equal(t, "cmd", byName["a"].Command)
	assert.Equal(t, "http://x", byName["b"].URL)
}

func TestConfig_RemoveServer_Found(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{"echo":{"command":"node"}}}`), 0644))
	ok, err := c.RemoveServer("echo")
	require.NoError(t, err)
	assert.True(t, ok)
	servers, err := c.Load()
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestConfig_RemoveServer_NotFound(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{}}`), 0644))
	ok, err := c.RemoveServer("missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConfig_UpdateSuppressedTools_AddNew(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{"echo":{"command":"node"}}}`), 0644))
	out, err := c.UpdateSuppressedTools("echo", "toolX", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"toolX"}, out)
}

func TestConfig_UpdateSuppressedTools_RemoveExisting(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{"echo":{"command":"node","anythingllm":{"suppressedTools":["toolX"]}}}}`), 0644))
	out, err := c.UpdateSuppressedTools("echo", "toolX", true)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestConfig_UpdateSuppressedTools_Idempotent_Add(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{"echo":{"command":"node","anythingllm":{"suppressedTools":["toolX"]}}}}`), 0644))
	out, err := c.UpdateSuppressedTools("echo", "toolX", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"toolX"}, out)
}

func TestConfig_UpdateSuppressedTools_Idempotent_Remove(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{"echo":{"command":"node"}}}`), 0644))
	out, err := c.UpdateSuppressedTools("echo", "toolX", true)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestConfig_UpdateSuppressedTools_ServerNotFound(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{}}`), 0644))
	out, err := c.UpdateSuppressedTools("missing", "toolX", false)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestConfig_GetSuppressedTools_Default(t *testing.T) {
	tmp := t.TempDir()
	c := NewConfig(tmp)
	require.NoError(t, os.MkdirAll(filepath.Dir(c.Path), 0755))
	require.NoError(t, os.WriteFile(c.Path, []byte(`{"mcpServers":{"echo":{"command":"node"}}}`), 0644))
	out := c.GetSuppressedTools("echo")
	assert.Empty(t, out)
}
