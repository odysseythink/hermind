package tool

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func echoHandler(ctx context.Context, args json.RawMessage) (string, error) {
	return string(args), nil
}

func failingHandler(ctx context.Context, args json.RawMessage) (string, error) {
	return "", errors.New("boom")
}

func TestRegisterAndDispatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "echo",
		Toolset: "test",
		Handler: echoHandler,
		Schema: ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        "echo",
				Description: "echo input",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	})

	out, err := r.Dispatch(context.Background(), "echo", json.RawMessage(`{"hi":"there"}`))
	require.NoError(t, err)
	assert.Equal(t, `{"hi":"there"}`, out)
}

func TestDispatchUnknownTool(t *testing.T) {
	r := NewRegistry()
	out, err := r.Dispatch(context.Background(), "missing", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "unknown tool")
	assert.Contains(t, out, `"error"`)
}

func TestDispatchHandlerError(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "fail",
		Handler: failingHandler,
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "fail"}},
	})

	out, err := r.Dispatch(context.Background(), "fail", nil)
	require.NoError(t, err)
	assert.Contains(t, out, `"error"`)
	assert.Contains(t, out, "boom")
}

func TestDefinitionsFiltersUnavailable(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "always",
		Handler: echoHandler,
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "always"}},
	})
	r.Register(&Entry{
		Name:    "hidden",
		Handler: echoHandler,
		CheckFn: func() bool { return false },
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "hidden"}},
	})

	defs := r.Definitions(nil)
	require.Len(t, defs, 1)
	assert.Equal(t, "always", defs[0].Function.Name)
}

func TestResultTruncation(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name: "big",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(make([]byte, 500)), nil
		},
		MaxResultChars: 100,
		Schema:         ToolDefinition{Type: "function", Function: FunctionDef{Name: "big"}},
	})

	out, err := r.Dispatch(context.Background(), "big", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "[truncated]")
	assert.LessOrEqual(t, len(out), 150) // truncation marker adds a bit
}

func TestConcurrentRegisterDispatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&Entry{
		Name:    "concurrent",
		Handler: echoHandler,
		Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: "concurrent"}},
	})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = r.Dispatch(context.Background(), "concurrent", json.RawMessage(`{}`))
		}()
		go func(n int) {
			defer func() { done <- struct{}{} }()
			r.Register(&Entry{
				Name:    nameForInt(n),
				Handler: echoHandler,
				Schema:  ToolDefinition{Type: "function", Function: FunctionDef{Name: nameForInt(n)}},
			})
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

func nameForInt(n int) string {
	return string(rune('a'+n)) + "tool"
}
