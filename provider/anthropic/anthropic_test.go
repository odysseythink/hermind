// provider/anthropic/anthropic_test.go
package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Anthropic) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p, err := New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "test-key",
		BaseURL:  srv.URL,
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)
	return srv, p.(*Anthropic)
}

func TestCompleteHappyPath(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, defaultAPIVersion, r.Header.Get("anthropic-version"))

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req messagesRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "claude-opus-4-6", req.Model)
		assert.Equal(t, "You are helpful.", req.System)
		require.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)

		// Send canned response
		resp := messagesResponse{
			ID:    "msg_01",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
			Content: []apiContentItem{
				{Type: "text", Text: "Hello back"},
			},
			StopReason: "end_turn",
			Usage:      apiUsage{InputTokens: 10, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := &provider.Request{
		Model:        "claude-opus-4-6",
		SystemPrompt: "You are helpful.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
		},
		MaxTokens: 1024,
	}

	resp, err := a.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, message.RoleAssistant, resp.Message.Role)
	assert.Equal(t, "Hello back", resp.Message.Content.Text())
	assert.Equal(t, "end_turn", resp.FinishReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
}

func TestCompleteMapsRateLimitError(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{
			Type:  "error",
			Error: apiError{Type: "rate_limit_error", Message: "rate limited"},
		})
	})

	_, err := a.Complete(context.Background(), &provider.Request{
		Model:    "claude-opus-4-6",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)

	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrRateLimit, pErr.Kind)
	assert.Equal(t, 429, pErr.StatusCode)
	assert.True(t, provider.IsRetryable(err))
}

func TestStreamHappyPath(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Write SSE events matching Anthropic format
		events := []string{
			`event: message_start
data: {"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-opus-4-6","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

`,
			`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

`,
			`event: content_block_stop
data: {"type":"content_block_stop","index":0}

`,
			`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

`,
			`event: message_stop
data: {"type":"message_stop"}

`,
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := a.Stream(context.Background(), &provider.Request{
		Model:    "claude-opus-4-6",
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)
	defer stream.Close()

	var text string
	var doneEvent *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.Type == provider.EventDone {
			doneEvent = ev
			break
		}
		if ev.Type == provider.EventDelta && ev.Delta != nil {
			text += ev.Delta.Content
		}
	}

	assert.Equal(t, "Hello world", text)
	require.NotNil(t, doneEvent)
	require.NotNil(t, doneEvent.Response)
	assert.Equal(t, "end_turn", doneEvent.Response.FinishReason)
	assert.Equal(t, 10, doneEvent.Response.Usage.InputTokens)
	assert.Equal(t, 5, doneEvent.Response.Usage.OutputTokens)
}

func TestCompleteEmitsTools(t *testing.T) {
	var capturedReq messagesRequest
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &capturedReq))

		resp := messagesResponse{
			ID:    "msg_01",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
			Content: []apiContentItem{
				{Type: "text", Text: "I'll read the file."},
				{Type: "tool_use", ID: "tool_01", Name: "read_file", Input: json.RawMessage(`{"path":"go.mod"}`)},
			},
			StopReason: "tool_use",
			Usage:      apiUsage{InputTokens: 20, OutputTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := &provider.Request{
		Model: "claude-opus-4-6",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		},
		Tools: []tool.ToolDefinition{
			{Type: "function", Function: tool.FunctionDef{
				Name:        "read_file",
				Description: "Read a file",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
			}},
		},
		MaxTokens: 1024,
	}

	resp, err := a.Complete(context.Background(), req)
	require.NoError(t, err)

	// Request should have passed tools
	require.Len(t, capturedReq.Tools, 1)
	assert.Equal(t, "read_file", capturedReq.Tools[0].Name)
	assert.Equal(t, "Read a file", capturedReq.Tools[0].Description)

	// Response should have tool_use block and finish_reason
	assert.Equal(t, "tool_use", resp.FinishReason)
	require.Equal(t, message.RoleAssistant, resp.Message.Role)
	blocks := resp.Message.Content.Blocks()
	require.Len(t, blocks, 2)
	assert.Equal(t, "text", blocks[0].Type)
	assert.Equal(t, "I'll read the file.", blocks[0].Text)
	assert.Equal(t, "tool_use", blocks[1].Type)
	assert.Equal(t, "tool_01", blocks[1].ToolUseID)
	assert.Equal(t, "read_file", blocks[1].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[1].ToolUseInput))
}

func TestCompleteSendsToolResult(t *testing.T) {
	var capturedReq messagesRequest
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &capturedReq))

		resp := messagesResponse{
			ID: "msg_02", Type: "message", Role: "assistant", Model: "claude-opus-4-6",
			Content:    []apiContentItem{{Type: "text", Text: "Done."}},
			StopReason: "end_turn",
			Usage:      apiUsage{InputTokens: 30, OutputTokens: 3},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// History: user message, assistant tool_use, user tool_result
	history := []message.Message{
		{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		{
			Role: message.RoleAssistant,
			Content: message.BlockContent([]message.ContentBlock{
				{
					Type:         "tool_use",
					ToolUseID:    "tool_01",
					ToolUseName:  "read_file",
					ToolUseInput: json.RawMessage(`{"path":"go.mod"}`),
				},
			}),
		},
		{
			Role: message.RoleUser,
			Content: message.BlockContent([]message.ContentBlock{
				{
					Type:       "tool_result",
					ToolUseID:  "tool_01",
					ToolResult: `{"content":"module x"}`,
				},
			}),
		},
	}

	req := &provider.Request{Model: "claude-opus-4-6", Messages: history, MaxTokens: 1024}
	resp, err := a.Complete(context.Background(), req)
	require.NoError(t, err)

	// Verify the request sent to the server has all 3 messages with correct content types
	require.Len(t, capturedReq.Messages, 3)
	assert.Equal(t, "user", capturedReq.Messages[0].Role)
	assert.Equal(t, "assistant", capturedReq.Messages[1].Role)
	assert.Equal(t, "user", capturedReq.Messages[2].Role)

	// Assistant turn should contain a tool_use block
	assistantContent := capturedReq.Messages[1].Content
	require.Len(t, assistantContent, 1)
	assert.Equal(t, "tool_use", assistantContent[0].Type)
	assert.Equal(t, "tool_01", assistantContent[0].ID)

	// User turn 3 should contain a tool_result block
	userResult := capturedReq.Messages[2].Content
	require.Len(t, userResult, 1)
	assert.Equal(t, "tool_result", userResult[0].Type)
	assert.Equal(t, "tool_01", userResult[0].ToolUseID)

	assert.Equal(t, "end_turn", resp.FinishReason)
}

func TestStreamToolUse(t *testing.T) {
	_, a := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		// SSE sequence for a tool_use response
		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_03\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":20,\"output_tokens\":0}}}\n\n",
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_02\",\"name\":\"read_file\",\"input\":{}}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\"}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"go.mod\\\"}\"}}\n\n",
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":8}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	stream, err := a.Stream(context.Background(), &provider.Request{
		Model: "claude-opus-4-6",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("read go.mod")},
		},
	})
	require.NoError(t, err)
	defer stream.Close()

	var doneEvent *provider.StreamEvent
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.Type == provider.EventDone {
			doneEvent = ev
			break
		}
	}

	require.NotNil(t, doneEvent)
	require.NotNil(t, doneEvent.Response)
	assert.Equal(t, "tool_use", doneEvent.Response.FinishReason)

	blocks := doneEvent.Response.Message.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.Equal(t, "tool_02", blocks[0].ToolUseID)
	assert.Equal(t, "read_file", blocks[0].ToolUseName)
	assert.JSONEq(t, `{"path":"go.mod"}`, string(blocks[0].ToolUseInput))
}
