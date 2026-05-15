// agent/conversation_test.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/pantheonadapter"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldCompress_DisabledOrZeroProtectLast(t *testing.T) {
	hist := []message.HermindMessage{{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("hi")}}

	assert.False(t, shouldCompress(hist, compression.CompressionConfig{Enabled: false, ProtectLast: 20}, nil, ""),
		"disabled compression must never trigger")
	assert.False(t, shouldCompress(hist, compression.CompressionConfig{Enabled: true, ProtectLast: 0}, nil, ""),
		"zero ProtectLast must never trigger")
}

func TestShouldCompress_MessageCountTrigger(t *testing.T) {
	cfg := compression.CompressionConfig{Enabled: true, ProtectLast: 5} // threshold = 15 messages
	hist := make([]message.HermindMessage, 16)
	for i := range hist {
		hist[i] = message.HermindMessage{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("x")}
	}
	// No resolver supplied — should still trigger via the count fallback.
	assert.True(t, shouldCompress(hist, cfg, nil, ""))
}

// TestShouldCompress_TokenBudgetTrigger covers fix #2: when the message
// count is below the legacy threshold but a single oversized paste pushes
// estimated tokens past Threshold * ContextLength, compression must fire.
func TestShouldCompress_TokenBudgetTrigger(t *testing.T) {
	cfg := compression.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		ProtectLast: 20, // count threshold = 60 messages, never reached here
	}
	// deepseek-chat has ContextLength=64000 in the resolver DB.
	// Budget = 0.5 * 64000 = 32000 tokens.
	// 150_000 chars => (150000+3)/4 = 37500 tokens > 32000.
	giant := strings.Repeat("x", 150_000)
	hist := []message.HermindMessage{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("intro")},
		{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("ack")},
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent(giant)},
	}

	assert.True(t, shouldCompress(hist, cfg, pantheonadapter.NewModelInfoResolver(), "deepseek-chat"),
		"token budget exceeded — must trigger compression")
}

func TestShouldCompress_TokenBudgetUnderLimit(t *testing.T) {
	cfg := compression.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		ProtectLast: 20,
	}
	hist := []message.HermindMessage{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("hi")},
		{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("hello")},
	}

	assert.False(t, shouldCompress(hist, cfg, pantheonadapter.NewModelInfoResolver(), "qwen-plus"),
		"well under both triggers — must not compress")
}

func TestShouldCompress_NilProviderNoTokenTrigger(t *testing.T) {
	cfg := compression.CompressionConfig{
		Enabled:     true,
		Threshold:   0.5,
		ProtectLast: 20,
	}
	hist := make([]message.HermindMessage, 5)
	for i := range hist {
		hist[i] = message.HermindMessage{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent(strings.Repeat("x", 1_000_000))}
	}
	// No resolver — token trigger silently no-ops, count trigger doesn't fire.
	assert.False(t, shouldCompress(hist, cfg, nil, ""))
}

// capturingFakeProvider records each request and returns pre-programmed responses.
type capturingFakeProvider struct {
	responses []*provider.Response
	idx       int
	lastReq   *core.Request
}

func (p *capturingFakeProvider) Provider() string { return "capturing" }
func (p *capturingFakeProvider) Model() string    { return "" }
func (p *capturingFakeProvider) Generate(context.Context, *core.Request) (*core.Response, error) {
	return nil, nil
}
func (p *capturingFakeProvider) GenerateObject(context.Context, *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, nil
}

func (p *capturingFakeProvider) Stream(_ context.Context, req *core.Request) (core.StreamResponse, error) {
	p.lastReq = req
	if p.idx >= len(p.responses) {
		return nil, fmt.Errorf("unexpected stream call #%d (have %d responses)", p.idx, len(p.responses))
	}
	resp := p.responses[p.idx]
	p.idx++
	return responseToStream(resp), nil
}

func toolUseResponse(toolID, toolName, args string) *provider.Response {
	return &provider.Response{
		Message: message.HermindMessage{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: []core.ContentParter{core.ToolCallPart{ID: toolID, Name: toolName, Arguments: args}},
		},
		FinishReason: "tool_use",
	}
}

func emptyTextResponse() *provider.Response {
	return &provider.Response{
		Message: message.HermindMessage{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: core.NewTextContent("\n\n"),
		},
		FinishReason: "end_turn",
	}
}

func textResponse(text string) *provider.Response {
	return &provider.Response{
		Message: message.HermindMessage{
			Role:    core.MESSAGE_ROLE_ASSISTANT,
			Content: core.NewTextContent(text),
		},
		FinishReason: "end_turn",
	}
}

func TestRunConversation_EmptyResponseAfterToolCall_Retries(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo",
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: core.ToolDefinition{
			Name:       "echo",
			Parameters: core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	p := &capturingFakeProvider{
		responses: []*provider.Response{
			toolUseResponse("t1", "echo", `{}`),
			emptyTextResponse(),
			textResponse("Gold price is $2000/oz"),
		},
	}
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{UserMessage: "search gold price"})
	require.NoError(t, err)
	assert.Equal(t, "Gold price is $2000/oz", result.Response.Text())
	assert.Equal(t, 3, result.Iterations)

	require.Len(t, result.Messages, 6)
	assert.Equal(t, core.MESSAGE_ROLE_USER, result.Messages[4].Role)
	assert.Contains(t, result.Messages[4].Text(), "Please provide your answer")
}

func TestRunConversation_EmptyResponseTwice_ThenSucceeds(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo",
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: core.ToolDefinition{
			Name:       "echo",
			Parameters: core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	p := &capturingFakeProvider{
		responses: []*provider.Response{
			toolUseResponse("t1", "echo", `{}`),
			emptyTextResponse(),
			emptyTextResponse(),
			textResponse("Answer found"),
		},
	}
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{UserMessage: "search"})
	require.NoError(t, err)
	assert.Equal(t, "Answer found", result.Response.Text())
	assert.Equal(t, 4, result.Iterations)
}

func TestRunConversation_EmptyResponseExceedsRetries(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo",
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: core.ToolDefinition{
			Name:       "echo",
			Parameters: core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	p := &capturingFakeProvider{
		responses: []*provider.Response{
			toolUseResponse("t1", "echo", `{}`),
			emptyTextResponse(),
			emptyTextResponse(),
			emptyTextResponse(),
		},
	}
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{UserMessage: "search"})
	require.NoError(t, err)
	assert.Equal(t, "", strings.TrimSpace(result.Response.Text()))
}

func TestRunConversation_FirstTurnEmpty_NoRetry(t *testing.T) {
	p := &capturingFakeProvider{
		responses: []*provider.Response{
			emptyTextResponse(),
		},
	}
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{UserMessage: "hi"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Iterations)
	assert.Equal(t, "", strings.TrimSpace(result.Response.Text()))
}

func TestRunConversation_EmptyRetry_StripsTools(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo",
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: core.ToolDefinition{
			Name:       "echo",
			Parameters: core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	p := &capturingFakeProvider{
		responses: []*provider.Response{
			toolUseResponse("t1", "echo", `{}`),
			emptyTextResponse(),
			textResponse("OK"),
		},
	}
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	_, err := e.RunConversation(context.Background(), &RunOptions{UserMessage: "search"})
	require.NoError(t, err)

	require.NotNil(t, p.lastReq)
	assert.Empty(t, p.lastReq.Tools, "retry request should have no tools")
}

func TestRunConversation_NonEmptyAfterToolCall_NoRetry(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "echo",
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
		Schema: core.ToolDefinition{
			Name:       "echo",
			Parameters: core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	p := &capturingFakeProvider{
		responses: []*provider.Response{
			toolUseResponse("t1", "echo", `{}`),
			textResponse("Done"),
		},
	}
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{UserMessage: "search"})
	require.NoError(t, err)
	assert.Equal(t, "Done", result.Response.Text())
	assert.Equal(t, 2, result.Iterations)
}

func TestRunConversation_ToolDispatchError(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Entry{
		Name: "fail_tool",
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", fmt.Errorf("tool exploded")
		},
		Schema: core.ToolDefinition{
			Name:       "fail_tool",
			Parameters: core.MustSchemaFromJSON([]byte(`{"type":"object"}`)),
		},
	})

	p := &capturingFakeProvider{
		responses: []*provider.Response{
			toolUseResponse("t1", "fail_tool", `{}`),
			textResponse("Handled the error"),
		},
	}
	e := NewEngineWithTools(p, nil, reg, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{UserMessage: "run fail"})
	require.NoError(t, err)
	assert.Equal(t, "Handled the error", result.Response.Text())

	// The tool_result should contain the error
	require.Len(t, result.Messages, 4)
	assert.Contains(t, result.Messages[2].Text(), "tool exploded")
}

func TestRunConversation_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	p := newFakeStreamingProvider("should not get here")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	_, err := e.RunConversation(ctx, &RunOptions{UserMessage: "hi"})
	assert.ErrorIs(t, err, context.Canceled)
}
