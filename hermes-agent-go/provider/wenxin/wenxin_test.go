// provider/wenxin/wenxin_test.go
package wenxin

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

// newMockBaidu builds an httptest.Server that handles both OAuth and chat endpoints.
func newMockBaidu(t *testing.T, chatHandler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/2.0/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "mock_token",
			"expires_in":   3600,
		})
	})
	mux.Handle("/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/", chatHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNewRejectsBadAPIKey(t *testing.T) {
	_, err := New(config.ProviderConfig{Provider: "wenxin", APIKey: "no_colon"})
	assert.Error(t, err)
}

func TestCompleteHappyPath(t *testing.T) {
	srv := newMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/wenxinworkshop/chat/ernie-speed")
		assert.Equal(t, "mock_token", r.URL.Query().Get("access_token"))
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		require.NoError(t, json.Unmarshal(body, &req))
		require.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "hi", req.Messages[0].Content)

		_ = json.NewEncoder(w).Encode(chatResponse{
			ID:     "wxid_01",
			Result: "你好",
			Usage:  usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
		})
	})

	p, err := New(config.ProviderConfig{
		Provider: "wenxin",
		APIKey:   "api:secret",
		BaseURL:  srv.URL,
		Model:    "ernie-speed",
	})
	require.NoError(t, err)

	resp, err := p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.NoError(t, err)
	assert.Equal(t, "你好", resp.Message.Content.Text())
	assert.Equal(t, 3, resp.Usage.InputTokens)
	assert.Equal(t, 2, resp.Usage.OutputTokens)
}

func TestCompleteMapsBaiduErrorCode(t *testing.T) {
	srv := newMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatResponse{
			ErrorCode: 17,
			ErrorMsg:  "quota exceeded",
		})
	})

	p, _ := New(config.ProviderConfig{
		Provider: "wenxin", APIKey: "api:secret", BaseURL: srv.URL, Model: "ernie-speed",
	})

	_, err := p.Complete(context.Background(), &provider.Request{
		Messages: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
	})
	require.Error(t, err)
	var pErr *provider.Error
	require.ErrorAs(t, err, &pErr)
	assert.Equal(t, provider.ErrRateLimit, pErr.Kind)
}

func TestStreamHappyPath(t *testing.T) {
	srv := newMockBaidu(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		events := []string{
			`data: {"id":"w1","result":"你","is_end":false}` + "\n\n",
			`data: {"id":"w1","result":"好","is_end":false}` + "\n\n",
			`data: {"id":"w1","result":"","is_end":true,"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}` + "\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	})

	p, _ := New(config.ProviderConfig{
		Provider: "wenxin", APIKey: "api:secret", BaseURL: srv.URL, Model: "ernie-speed",
	})
	stream, err := p.Stream(context.Background(), &provider.Request{
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
	assert.Equal(t, "你好", text)
	require.NotNil(t, done.Response)
	assert.Equal(t, 3, done.Response.Usage.InputTokens)
	assert.Equal(t, 2, done.Response.Usage.OutputTokens)
}
