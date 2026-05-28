package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHypervisor_BootCachesTools(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.mcps["echo"]
	require.True(t, ok)
	require.Len(t, client.tools, 3)
	assert.NotEmpty(t, client.schemaByName["echo"])
}

func TestHypervisor_GetToolSchema_Found(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ts, err := h.GetToolSchema("echo", "echo")
	require.NoError(t, err)
	require.NotNil(t, ts)
	assert.Equal(t, "echo", ts.Name)
	assert.NotEmpty(t, ts.InputSchema)
}

func TestHypervisor_GetToolSchema_ServerNotFound(t *testing.T) {
	h, _ := newTestHypervisor(t)
	ts, err := h.GetToolSchema("ghost", "x")
	assert.Nil(t, ts)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_GetToolSchema_ToolNotFound(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	ts, err := h.GetToolSchema("echo", "nope")
	assert.Nil(t, ts)
	assert.ErrorIs(t, err, ErrToolNotFound)
}

func TestHypervisor_GetToolSchema_ToolWithoutInputSchema(t *testing.T) {
	h, _ := newTestHypervisor(t)
	// Manually inject a client with a tool that has no inputSchema.
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

	ts, err := h.GetToolSchema("no-schema", "noop")
	require.NoError(t, err)
	require.NotNil(t, ts)
	assert.Equal(t, "noop", ts.Name)
	assert.Empty(t, ts.InputSchema)
}

func TestHypervisor_GetToolSchema_AfterToggleOff(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	_, err := h.GetToolSchema("echo", "echo")
	require.NoError(t, err)

	ok, err := h.ToggleServer(ctx, "echo")
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = h.GetToolSchema("echo", "echo")
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestHypervisor_Servers_UsesCachedTools(t *testing.T) {
	h, tmp := newTestHypervisor(t)
	seedEchoConfig(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, h.Boot(ctx))

	// Servers() should use the cached tools, not call ListTools again.
	servers, err := h.Servers(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Len(t, servers[0].Tools, 3)
}

func TestHypervisor_GetToolSchema_ErrorsAreWrapped(t *testing.T) {
	h, _ := newTestHypervisor(t)

	_, err := h.GetToolSchema("missing", "tool")
	assert.True(t, errors.Is(err, ErrServerNotFound))
	assert.Contains(t, err.Error(), "missing")

	h.mu.Lock()
	h.mcps["srv"] = &activeClient{
		tools:        []ToolSchema{},
		schemaByName: map[string]json.RawMessage{},
	}
	h.mu.Unlock()

	_, err = h.GetToolSchema("srv", "tool")
	assert.True(t, errors.Is(err, ErrToolNotFound))
	assert.Contains(t, err.Error(), "srv")
}
