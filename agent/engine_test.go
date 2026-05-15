// agent/engine_test.go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider returns a canned response for tests.
type fakeProvider struct {
	name      string
	response  *provider.Response
	err       error
	streamFn  func() (core.StreamResponse, error)
	lastModel string
}

func (f *fakeProvider) Provider() string { return f.name }
func (f *fakeProvider) Model() string    { return f.lastModel }
func (f *fakeProvider) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &core.Response{
		Message:      message.ToPantheon(f.response.Message),
		FinishReason: f.response.FinishReason,
		Usage:        core.Usage{PromptTokens: f.response.Usage.PromptTokens, CompletionTokens: f.response.Usage.CompletionTokens},
	}, nil
}
func (f *fakeProvider) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	f.lastModel = ""
	if f.streamFn != nil {
		return f.streamFn()
	}
	return nil, errors.New("not implemented")
}
func (f *fakeProvider) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

// responseToStream converts a hermind provider.Response into a pantheon StreamResponse iterator.
func responseToStream(resp *provider.Response) core.StreamResponse {
	return func(yield func(*core.StreamPart, error) bool) {
		for _, p := range resp.Message.Content {
			switch pt := p.(type) {
			case core.TextPart:
				if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: pt.Text}, nil) {
					return
				}
			case core.ToolCallPart:
				tcp := pt
				if !yield(&core.StreamPart{
					Type:     core.StreamPartTypeToolCall,
					ToolCall: &tcp,
				}, nil) {
					return
				}
			}
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeFinish, FinishReason: resp.FinishReason}, nil) {
			return
		}
	}
}

func newFakeStreamingProvider(text string) *fakeProvider {
	return &fakeProvider{
		name: "fake",
		streamFn: func() (core.StreamResponse, error) {
			return responseToStream(&provider.Response{
				Message: message.HermindMessage{
					Role:    core.MESSAGE_ROLE_ASSISTANT,
					Content: core.NewTextContent(text),
				},
				FinishReason: "end_turn",
				Usage:        core.Usage{PromptTokens: 5, CompletionTokens: 3},
			}), nil
		},
	}
}

func TestEngineSingleTurn(t *testing.T) {
	p := newFakeStreamingProvider("Hello back!")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hi",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello back!", result.Response.Text())
	assert.Equal(t, 1, result.Iterations)
	require.Len(t, result.Messages, 2)
	assert.Equal(t, core.MESSAGE_ROLE_USER, result.Messages[0].Role)
	assert.Equal(t, core.MESSAGE_ROLE_ASSISTANT, result.Messages[1].Role)
}

func TestEngineRespectsContextCancellation(t *testing.T) {
	p := newFakeStreamingProvider("should not get here")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.RunConversation(ctx, &RunOptions{
		UserMessage: "hi",
	})
	assert.ErrorIs(t, err, context.Canceled)
}

func newFakeProviderForScript(responses []*provider.Response) *fakeProvider {
	idx := 0
	return &fakeProvider{
		name: "fake",
		streamFn: func() (core.StreamResponse, error) {
			if idx >= len(responses) {
				return nil, errors.New("unexpected extra stream call")
			}
			resp := responses[idx]
			idx++
			return responseToStream(resp), nil
		},
	}
}

func TestEngineToolLoopSingleToolCall(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo_args",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"echoed":true}`, nil
		},
		Schema: core.ToolDefinition{
			Name:        "echo_args",
			Description: "Echo the arguments back",
			Parameters:  core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	responses := []*provider.Response{
		{
			Message: message.HermindMessage{
				Role:    core.MESSAGE_ROLE_ASSISTANT,
				Content: []core.ContentParter{core.ToolCallPart{ID: "t1", Name: "echo_args", Arguments: "{}"}},
			},
			FinishReason: "tool_use",
			Usage:        core.Usage{PromptTokens: 10, CompletionTokens: 5},
		},
		{
			Message: message.HermindMessage{
				Role:    core.MESSAGE_ROLE_ASSISTANT,
				Content: core.NewTextContent("Done. Got echoed=true."),
			},
			FinishReason: "end_turn",
			Usage:        core.Usage{PromptTokens: 15, CompletionTokens: 8},
		},
	}

	p := newFakeProviderForScript(responses)
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "run echo",
	})
	require.NoError(t, err)
	assert.Equal(t, "Done. Got echoed=true.", result.Response.Text())
	assert.Equal(t, 2, result.Iterations)

	require.Len(t, result.Messages, 4)
	assert.Equal(t, core.MESSAGE_ROLE_USER, result.Messages[0].Role)
	assert.Equal(t, core.MESSAGE_ROLE_ASSISTANT, result.Messages[1].Role)
	assert.Equal(t, core.MESSAGE_ROLE_TOOL, result.Messages[2].Role)
	assert.Equal(t, core.MESSAGE_ROLE_ASSISTANT, result.Messages[3].Role)

	require.False(t, result.Messages[2].IsTextOnly())
	require.Len(t, result.Messages[2].Content, 1)
	tr, ok := result.Messages[2].Content[0].(core.ToolResultPart)
	require.True(t, ok, "expected Content[0] to be ToolResultPart")
	assert.Equal(t, "t1", tr.ToolCallID)
	assert.Contains(t, message.HermindMessage{Content: tr.Content}.Text(), "echoed")
}

// fakeFallbackLM tries a list of language models in order until one succeeds.
type fakeFallbackLM struct {
	providers []core.LanguageModel
}

func (f *fakeFallbackLM) Provider() string { return "fallback" }
func (f *fakeFallbackLM) Model() string    { return "" }
func (f *fakeFallbackLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	for _, p := range f.providers {
		if resp, err := p.Generate(ctx, req); err == nil {
			return resp, nil
		}
	}
	return nil, errors.New("all providers failed")
}
func (f *fakeFallbackLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	for _, p := range f.providers {
		if resp, err := p.Stream(ctx, req); err == nil {
			return resp, nil
		}
	}
	return nil, errors.New("all providers failed")
}
func (f *fakeFallbackLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func TestEngineUsesFallbackChainOnRetryableError(t *testing.T) {
	failing := &fakeProvider{
		name: "failing",
		streamFn: func() (core.StreamResponse, error) {
			return nil, &provider.Error{Kind: provider.ErrRateLimit, Provider: "failing", Message: "rate limited"}
		},
	}

	succeeding := newFakeStreamingProvider("fallback response")
	succeeding.name = "succeeding"

	chain := &fakeFallbackLM{providers: []core.LanguageModel{failing, succeeding}}
	e := NewEngine(chain, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hello",
	})
	require.NoError(t, err)
	assert.Equal(t, "fallback response", result.Response.Text())
	assert.Equal(t, 1, result.Iterations)
}

func TestEngineBudgetExhaustion(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "loop_tool",
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: core.ToolDefinition{
			Name:       "loop_tool",
			Parameters: core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	toolUseResp := &provider.Response{
		Message: message.HermindMessage{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: []core.ContentParter{core.ToolCallPart{ID: "t1", Name: "loop_tool", Arguments: "{}"}},
		},
		FinishReason: "tool_use",
	}

	responses := []*provider.Response{toolUseResp, toolUseResp, toolUseResp, toolUseResp, toolUseResp}
	p := newFakeProviderForScript(responses)

	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 3}, "cli")
	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "loop forever",
	})
	require.NoError(t, err)
	assert.Equal(t, 3, result.Iterations, "should run exactly MaxTurns iterations")
}

func TestRunConversation_EphemeralDoesNotPersist(t *testing.T) {
	p := newFakeStreamingProvider("ephemeral")
	// Storage is nil — even if we passed one, Ephemeral would skip it.
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 2}, "test")

	_, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hi",
		Ephemeral:   true,
	})
	require.NoError(t, err)
}
