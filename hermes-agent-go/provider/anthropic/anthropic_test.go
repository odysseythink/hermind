// provider/anthropic/anthropic_test.go
package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
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
