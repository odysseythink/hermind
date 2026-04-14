// agent/engine_test.go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider returns a canned response for tests.
type fakeProvider struct {
	name     string
	response *provider.Response
	err      error
	streamFn func() (provider.Stream, error)
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}
func (f *fakeProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	if f.streamFn != nil {
		return f.streamFn()
	}
	return nil, errors.New("not implemented")
}
func (f *fakeProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (f *fakeProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (f *fakeProvider) Available() bool                             { return true }

// fakeStream returns a single delta then Done.
type fakeStream struct {
	events []*provider.StreamEvent
	idx    int
}

func (s *fakeStream) Recv() (*provider.StreamEvent, error) {
	if s.idx >= len(s.events) {
		return nil, io.EOF
	}
	ev := s.events[s.idx]
	s.idx++
	return ev, nil
}
func (s *fakeStream) Close() error { return nil }

func newFakeStreamingProvider(text string) *fakeProvider {
	return &fakeProvider{
		name: "fake",
		streamFn: func() (provider.Stream, error) {
			return &fakeStream{
				events: []*provider.StreamEvent{
					{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: text}},
					{
						Type: provider.EventDone,
						Response: &provider.Response{
							Message: message.Message{
								Role:    message.RoleAssistant,
								Content: message.TextContent(text),
							},
							FinishReason: "end_turn",
							Usage:        message.Usage{InputTokens: 5, OutputTokens: 3},
						},
					},
				},
			}, nil
		},
	}
}

func TestEngineSingleTurn(t *testing.T) {
	p := newFakeStreamingProvider("Hello back!")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hi",
		SessionID:   "test-session",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello back!", result.Response.Content.Text())
	assert.Equal(t, 1, result.Iterations)
	require.Len(t, result.Messages, 2)
	assert.Equal(t, message.RoleUser, result.Messages[0].Role)
	assert.Equal(t, message.RoleAssistant, result.Messages[1].Role)
}

func TestEngineRespectsContextCancellation(t *testing.T) {
	p := newFakeStreamingProvider("should not get here")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before run

	_, err := e.RunConversation(ctx, &RunOptions{
		UserMessage: "hi",
		SessionID:   "cancelled-session",
	})
	assert.ErrorIs(t, err, context.Canceled)
}

// newFakeProviderForScript replays a fixed sequence of responses on each Stream call.
func newFakeProviderForScript(responses []*provider.Response) *fakeProvider {
	idx := 0
	return &fakeProvider{
		name: "fake",
		streamFn: func() (provider.Stream, error) {
			if idx >= len(responses) {
				return nil, errors.New("unexpected extra stream call")
			}
			resp := responses[idx]
			idx++
			return &fakeStream{
				events: []*provider.StreamEvent{
					{Type: provider.EventDone, Response: resp},
				},
			}, nil
		},
	}
}

func TestEngineToolLoopSingleToolCall(t *testing.T) {
	// Prepare a registry with a fake "echo_args" tool
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo_args",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"echoed":true}`, nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "echo_args",
				Description: "Echo the arguments back",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	})

	// Provider returns: turn 1 = tool_use, turn 2 = final text
	responses := []*provider.Response{
		{
			Message: message.Message{
				Role: message.RoleAssistant,
				Content: message.BlockContent([]message.ContentBlock{
					{
						Type:         "tool_use",
						ToolUseID:    "t1",
						ToolUseName:  "echo_args",
						ToolUseInput: json.RawMessage(`{}`),
					},
				}),
			},
			FinishReason: "tool_use",
			Usage:        message.Usage{InputTokens: 10, OutputTokens: 5},
		},
		{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent("Done. Got echoed=true."),
			},
			FinishReason: "end_turn",
			Usage:        message.Usage{InputTokens: 15, OutputTokens: 8},
		},
	}

	p := newFakeProviderForScript(responses)
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "run echo",
		SessionID:   "tool-test",
	})
	require.NoError(t, err)
	assert.Equal(t, "Done. Got echoed=true.", result.Response.Content.Text())
	assert.Equal(t, 2, result.Iterations)

	// History should have: user, assistant(tool_use), user(tool_result), assistant(text) = 4
	require.Len(t, result.Messages, 4)
	assert.Equal(t, message.RoleUser, result.Messages[0].Role)
	assert.Equal(t, message.RoleAssistant, result.Messages[1].Role)
	assert.Equal(t, message.RoleUser, result.Messages[2].Role) // tool_result
	assert.Equal(t, message.RoleAssistant, result.Messages[3].Role)

	// The tool_result message should be a BlockContent with a tool_result block
	require.False(t, result.Messages[2].Content.IsText())
	blocks := result.Messages[2].Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_result", blocks[0].Type)
	assert.Equal(t, "t1", blocks[0].ToolUseID)
	assert.Contains(t, blocks[0].ToolResult, "echoed")
}

func TestEngineUsesFallbackChainOnRetryableError(t *testing.T) {
	// failingProvider returns a retryable ErrRateLimit from Stream
	failing := &fakeProvider{
		name: "failing",
		streamFn: func() (provider.Stream, error) {
			return nil, &provider.Error{Kind: provider.ErrRateLimit, Provider: "failing", Message: "rate limited"}
		},
	}

	// succeedingProvider returns a normal text response
	succeeding := newFakeStreamingProvider("fallback response")
	succeeding.name = "succeeding"

	chain := provider.NewFallbackChain([]provider.Provider{failing, succeeding})
	e := NewEngine(chain, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hello",
		SessionID:   "fallback-test",
	})
	require.NoError(t, err)
	assert.Equal(t, "fallback response", result.Response.Content.Text())
	assert.Equal(t, 1, result.Iterations)
}

func TestEngineBudgetExhaustion(t *testing.T) {
	// Provider that ALWAYS returns a tool_use — should exhaust budget
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "loop_tool",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:       "loop_tool",
				Parameters: json.RawMessage(`{"type":"object"}`),
			},
		},
	})

	toolUseResp := &provider.Response{
		Message: message.Message{
			Role: message.RoleAssistant,
			Content: message.BlockContent([]message.ContentBlock{
				{Type: "tool_use", ToolUseID: "t1", ToolUseName: "loop_tool", ToolUseInput: json.RawMessage(`{}`)},
			}),
		},
		FinishReason: "tool_use",
	}

	// Script of 10 tool_use responses — budget is 3, so only 3 should execute
	responses := []*provider.Response{toolUseResp, toolUseResp, toolUseResp, toolUseResp, toolUseResp}
	p := newFakeProviderForScript(responses)

	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 3}, "cli")
	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "loop forever",
		SessionID:   "budget-test",
	})
	// Budget exhaustion is not an error — it returns the partial result
	require.NoError(t, err)
	assert.Equal(t, 3, result.Iterations, "should run exactly MaxTurns iterations")
}
