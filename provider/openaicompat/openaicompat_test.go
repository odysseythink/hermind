// provider/openaicompat/openaicompat_test.go
package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{
		BaseURL:      srv.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		ProviderName: "test",
	})
	require.NoError(t, err)
	return srv, c
}

func TestCompleteHappyPath(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "test-model", req.Model)
		// system prompt should be first message
		require.NotEmpty(t, req.Messages)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "Be helpful.", req.Messages[0].Content)

		resp := chatResponse{
			ID:    "chat-001",
			Model: "test-model",
			Choices: []apiChoice{{
				Index: 0,
				Message: apiMessage{
					Role:    "assistant",
					Content: "Hello back",
				},
				FinishReason: "stop",
			}},
			Usage: apiUsage{PromptTokens: 10, CompletionTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	resp, err := c.Complete(context.Background(), &provider.Request{
		Model:        "test-model",
		SystemPrompt: "Be helpful.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
		},
		MaxTokens: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello back", resp.Message.Content.Text())
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
}

func TestCompleteHandlesToolCall(t *testing.T) {
	var captured chatRequest
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := chatResponse{
			ID:    "chat-002",
			Model: "test-model",
			Choices: []apiChoice{{
				Index: 0,
				Message: apiMessage{
					Role:    "assistant",
					Content: nil,
					ToolCalls: []apiToolCall{{
						ID:   "call_01",
						Type: "function",
						Function: apiFunctionCall{
							Name:      "read_file",
							Arguments: `{"path":"go.mod"}`,
						},
					}},
				},
				FinishReason: "tool_calls",
			}},
			Usage: apiUsage{PromptTokens: 20, CompletionTokens: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	resp, err := c.Complete(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		},
		Tools: []tool.ToolDefinition{{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "read_file",
				Description: "Read a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
		MaxTokens: 100,
	})
	require.NoError(t, err)

	// Request should have included the tool
	require.Len(t, captured.Tools, 1)
	assert.Equal(t, "read_file", captured.Tools[0].Function.Name)

	// Response should have tool_use block
	assert.Equal(t, "tool_calls", resp.FinishReason)
	blocks := resp.Message.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.Equal(t, "call_01", blocks[0].ToolUseID)
	assert.Equal(t, "read_file", blocks[0].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[0].ToolUseInput))
}

func TestCompleteSendsToolResult(t *testing.T) {
	var captured chatRequest
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := chatResponse{
			ID:    "chat-003",
			Model: "test-model",
			Choices: []apiChoice{{
				Index: 0,
				Message: apiMessage{Role: "assistant", Content: "Done."},
				FinishReason: "stop",
			}},
			Usage: apiUsage{PromptTokens: 30, CompletionTokens: 3},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		{
			Role: message.RoleAssistant,
			Content: message.BlockContent([]message.ContentBlock{
				{
					Type:         "tool_use",
					ToolUseID:    "call_01",
					ToolUseName:  "read_file",
					ToolUseInput: json.RawMessage(`{"path":"go.mod"}`),
				},
			}),
		},
		{
			Role: message.RoleUser,
			Content: message.BlockContent([]message.ContentBlock{
				{
					Type:        "tool_result",
					ToolUseID:   "call_01",
					ToolUseName: "read_file",
					ToolResult:  `{"content":"module x"}`,
				},
			}),
		},
	}
	_, err := c.Complete(context.Background(), &provider.Request{Model: "test-model", Messages: history})
	require.NoError(t, err)

	// Verify the wire messages: user, assistant(with tool_calls), tool(with tool_call_id)
	require.Len(t, captured.Messages, 3)
	assert.Equal(t, "user", captured.Messages[0].Role)
	assert.Equal(t, "assistant", captured.Messages[1].Role)
	require.Len(t, captured.Messages[1].ToolCalls, 1)
	assert.Equal(t, "call_01", captured.Messages[1].ToolCalls[0].ID)
	assert.Equal(t, "tool", captured.Messages[2].Role)
	assert.Equal(t, "call_01", captured.Messages[2].ToolCallID)
	assert.Equal(t, "read_file", captured.Messages[2].Name)
	assert.Equal(t, `{"content":"module x"}`, captured.Messages[2].Content)
}

func TestCompleteMapsRateLimit(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Error: apiError{Type: "rate_limit_exceeded", Message: "too many requests"},
		})
	})

	_, err := c.Complete(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)
	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrRateLimit, pErr.Kind)
	assert.True(t, provider.IsRetryable(err))
}

func TestCompleteMapsContextTooLong(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Error: apiError{Type: "invalid_request_error", Code: "context_length_exceeded", Message: "maximum context length is 128000"},
		})
	})

	_, err := c.Complete(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)
	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrContextTooLong, pErr.Kind)
}

func TestStreamHappyPath(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		events := []string{
			`data: {"id":"chat-004","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}` + "\n\n",
			`data: {"id":"chat-004","model":"test-model","choices":[{"index":0,"delta":{"content":" there"}}]}` + "\n\n",
			`data: {"id":"chat-004","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := c.Stream(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)
	defer stream.Close()

	var text string
	var done *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if ev.Type == provider.EventDone {
			done = ev
			break
		}
		if ev.Delta != nil {
			text += ev.Delta.Content
		}
	}
	assert.Equal(t, "Hi there", text)
	require.NotNil(t, done.Response)
	assert.Equal(t, "stop", done.Response.FinishReason)
	assert.Equal(t, 5, done.Response.Usage.InputTokens)
	assert.Equal(t, 2, done.Response.Usage.OutputTokens)
}

func TestStreamToolCall(t *testing.T) {
	_, c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Tool call streamed in pieces: id+name first, then arguments in 2 fragments
		events := []string{
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_09","type":"function","function":{"name":"read_file","arguments":""}}]}}]}` + "\n\n",
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]}}]}` + "\n\n",
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"go.mod\"}"}}]}}]}` + "\n\n",
			`data: {"id":"c1","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":6,"total_tokens":16}}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := c.Stream(context.Background(), &provider.Request{
		Model: "test-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("read go.mod")}},
	})
	require.NoError(t, err)
	defer stream.Close()

	var done *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if ev.Type == provider.EventDone {
			done = ev
			break
		}
	}
	require.NotNil(t, done.Response)
	blocks := done.Response.Message.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.Equal(t, "call_09", blocks[0].ToolUseID)
	assert.Equal(t, "read_file", blocks[0].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[0].ToolUseInput))
	assert.Equal(t, "tool_calls", done.Response.FinishReason)
}
