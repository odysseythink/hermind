package flow

import (
	"context"
	"fmt"
	"testing"

	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

type mockLM struct {
	generateFn func(ctx context.Context, req *core.Request) (*core.Response, error)
}

func (m *mockLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, req)
	}
	return &core.Response{Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "mocked")}, nil
}
func (m *mockLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockLM) Provider() string { return "mock" }
func (m *mockLM) Model() string    { return "mock-model" }

func TestLLMInstruction_HappyPath_CallsLM(t *testing.T) {
	called := false
	lm := &mockLM{
		generateFn: func(ctx context.Context, req *core.Request) (*core.Response, error) {
			called = true
			return &core.Response{Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "result")}, nil
		},
	}
	fc := &Context{LM: lm, Variables: map[string]string{}, Emit: func(string) {}}
	out, err := ExecuteLLMInstruction(context.Background(), fc, map[string]any{
		"instruction": "say hello",
	})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "result", out)
}

func TestLLMInstruction_InterpolatesInstruction(t *testing.T) {
	var received string
	lm := &mockLM{
		generateFn: func(ctx context.Context, req *core.Request) (*core.Response, error) {
			if len(req.Messages) > 0 {
			received = req.Messages[0].Text()
		}
			return &core.Response{Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "ok")}, nil
		},
	}
	fc := &Context{LM: lm, Variables: map[string]string{"name": "alice"}, Emit: func(string) {}}
	_, _ = ExecuteLLMInstruction(context.Background(), fc, map[string]any{
		"instruction": "greet {{name}}",
	})
	require.Equal(t, "greet alice", received)
}

func TestLLMInstruction_LMError_Returns(t *testing.T) {
	lm := &mockLM{
		generateFn: func(ctx context.Context, req *core.Request) (*core.Response, error) {
			return nil, fmt.Errorf("model down")
		},
	}
	fc := &Context{LM: lm, Variables: map[string]string{}, Emit: func(string) {}}
	_, err := ExecuteLLMInstruction(context.Background(), fc, map[string]any{
		"instruction": "test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "model down")
}

func TestLLMInstruction_EmptyInstruction_ReturnsError(t *testing.T) {
	fc := &Context{LM: &mockLM{}, Variables: map[string]string{}, Emit: func(string) {}}
	_, err := ExecuteLLMInstruction(context.Background(), fc, map[string]any{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "instruction is required")
}

func TestLLMInstruction_NilLM_ReturnsError(t *testing.T) {
	fc := &Context{LM: nil, Variables: map[string]string{}, Emit: func(string) {}}
	_, err := ExecuteLLMInstruction(context.Background(), fc, map[string]any{
		"instruction": "test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "LLM not available")
}
