package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/stretchr/testify/require"
)

func TestMCPProjection_BuildsEntryFromToolPlugin(t *testing.T) {
	p := mcp.ToolPlugin{
		ServerName:    "test",
		ToolName:      "echo",
		QualifiedName: "test-echo",
		Description:   "Echo back args",
		InputSchema:   json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
		Call: func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"echoed": args["msg"]}, nil
		},
	}
	emit := func(string) {}
	e := mcpToolToEntry(p, emit)

	require.Equal(t, "test-echo", e.Name)
	require.Equal(t, "mcp", e.Toolset)
	require.Equal(t, "Echo back args", e.Description)
	require.NotNil(t, e.Schema.Parameters)
}

func TestMCPProjection_PassesArgsCorrectly(t *testing.T) {
	var got map[string]any
	p := mcp.ToolPlugin{
		ServerName:    "test",
		ToolName:      "echo",
		QualifiedName: "test-echo",
		Description:   "Echo back args",
		InputSchema:   json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
		Call: func(ctx context.Context, args map[string]any) (any, error) {
			got = args
			return map[string]any{"echoed": args["msg"]}, nil
		},
	}
	var emitted []string
	emit := func(m string) { emitted = append(emitted, m) }
	e := mcpToolToEntry(p, emit)

	out, err := e.Handler(context.Background(), json.RawMessage(`{"msg":"hello"}`))
	require.NoError(t, err)
	require.Equal(t, "hello", got["msg"])
	require.Contains(t, out, `"echoed":"hello"`)
	require.Len(t, emitted, 1)
	require.Contains(t, emitted[0], "test-echo")
}

func TestMCPProjection_PropagatesError(t *testing.T) {
	p := mcp.ToolPlugin{
		ServerName:    "test",
		ToolName:      "fail",
		QualifiedName: "test-fail",
		Description:   "Always fails",
		Call: func(ctx context.Context, args map[string]any) (any, error) {
			return nil, errors.New("mcp boom")
		},
	}
	e := mcpToolToEntry(p, func(string) {})
	out, err := e.Handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Contains(t, out, "error")
	require.Contains(t, out, "mcp boom")
}

func TestMCPProjection_EmitsStatusBeforeCall(t *testing.T) {
	p := mcp.ToolPlugin{
		ServerName:    "test",
		ToolName:      "noop",
		QualifiedName: "test-noop",
		Description:   "No-op",
		Call: func(ctx context.Context, args map[string]any) (any, error) {
			return "ok", nil
		},
	}
	var emitted []string
	emit := func(m string) { emitted = append(emitted, m) }
	e := mcpToolToEntry(p, emit)
	_, _ = e.Handler(context.Background(), nil)
	require.Len(t, emitted, 1)
	require.Contains(t, emitted[0], "test-noop")
}

func TestMCPProjection_EmptyInputSchema_NilParameters(t *testing.T) {
	p := mcp.ToolPlugin{
		ServerName:    "test",
		ToolName:      "bare",
		QualifiedName: "test-bare",
		Description:   "No schema",
		InputSchema:   nil,
		Call: func(ctx context.Context, args map[string]any) (any, error) {
			return "ok", nil
		},
	}
	e := mcpToolToEntry(p, func(string) {})
	require.Nil(t, e.Schema.Parameters)
}

func TestMCPProjection_MarshalsResult(t *testing.T) {
	p := mcp.ToolPlugin{
		ServerName:    "test",
		ToolName:      "complex",
		QualifiedName: "test-complex",
		Description:   "Returns complex object",
		Call: func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"nested": map[string]int{"a": 1}}, nil
		},
	}
	e := mcpToolToEntry(p, func(string) {})
	out, err := e.Handler(context.Background(), nil)
	require.NoError(t, err)
	require.Contains(t, out, "nested")
	require.Contains(t, out, "a")
}
