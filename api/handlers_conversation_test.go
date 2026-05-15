package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/pantheon/core"
)

func newTempStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestConversationGet_EmptyReturnsEmptyList(t *testing.T) {
	store := newTempStore(t)
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Version: "test",
		Storage: store,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body ConversationHistoryResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body.Messages)
}

func TestConversationGet_ReturnsAppendedMessages(t *testing.T) {
	store := newTempStore(t)
	require.NoError(t, store.AppendMessage(context.Background(), &storage.StoredMessage{
		Role: "user", Content: `{"text":"hi"}`,
	}))

	srv, _ := NewServer(&ServerOpts{
		Config: &config.Config{}, Version: "test", Storage: store,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var body ConversationHistoryResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body.Messages, 1)
	assert.Equal(t, "user", body.Messages[0].Role)
}

func TestConversationPost_Returns503WhenNoProvider(t *testing.T) {
	srv, _ := NewServer(&ServerOpts{
		Config: &config.Config{}, Version: "test", Storage: newTempStore(t),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/conversation/messages",
		strings.NewReader(`{"user_message":"hi"}`))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestOldSessionRoutesReturn404(t *testing.T) {
	srv, _ := NewServer(&ServerOpts{
		Config: &config.Config{}, Version: "test",
	})
	for _, path := range []string{"/api/sessions", "/api/sessions/abc", "/api/sessions/abc/messages"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path %s", path)
	}
}

func TestConversationCancel_NoOpWhenNoneInFlight(t *testing.T) {
	srv, _ := NewServer(&ServerOpts{Config: &config.Config{}, Version: "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/conversation/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestConversationMessagePut(t *testing.T) {
	store := newTempStore(t)
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Version: "test",
		Storage: store,
	})
	require.NoError(t, err)
	ctx := context.Background()

	msg := &storage.StoredMessage{Role: "user", Content: "hello"}
	require.NoError(t, store.AppendMessage(ctx, msg))

	body := strings.NewReader(`{"content":"updated"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/conversation/messages/"+strconv.FormatInt(msg.ID, 10), body)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	history, err := store.GetHistory(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "updated", history[0].Content)
}

func TestConversationMessageDelete(t *testing.T) {
	store := newTempStore(t)
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Version: "test",
		Storage: store,
	})
	require.NoError(t, err)
	ctx := context.Background()

	for _, content := range []string{"a", "b"} {
		msg := &storage.StoredMessage{Role: "user", Content: content}
		require.NoError(t, store.AppendMessage(ctx, msg))
	}

	history, _ := store.GetHistory(ctx, 10, 0)
	require.Len(t, history, 2)

	req := httptest.NewRequest(http.MethodDelete, "/api/conversation/messages/"+strconv.FormatInt(history[0].ID, 10), nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	history, _ = store.GetHistory(ctx, 10, 0)
	require.Len(t, history, 1)
	assert.Equal(t, "b", history[0].Content)
}

// recordingProvider captures whether Stream was called.
type recordingProvider struct {
	stubProvider
	mu           sync.Mutex
	streamCalled bool
}

func (r *recordingProvider) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	r.mu.Lock()
	r.streamCalled = true
	r.mu.Unlock()
	return func(yield func(*core.StreamPart, error) bool) {
		if !yield(&core.StreamPart{Type: core.StreamPartTypeTextDelta, TextDelta: "ok"}, nil) {
			return
		}
		if !yield(&core.StreamPart{Type: core.StreamPartTypeFinish, FinishReason: "stop"}, nil) {
			return
		}
	}, nil
}

func (r *recordingProvider) wasStreamCalled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.streamCalled
}

func TestConversationPost_IgnoresBodyModel(t *testing.T) {
	store := newTempStore(t)
	rec := &recordingProvider{
		stubProvider: stubProvider{
			resp: &core.Response{
				Message: core.Message{
					Role:    core.MESSAGE_ROLE_ASSISTANT,
					Content: []core.ContentParter{core.TextPart{Text: "ok"}},
				},
			},
		},
	}

	srv, err := NewServer(&ServerOpts{
		Config: &config.Config{
			Model: "anthropic/claude-opus-4-6",
		},
		Version: "test",
		Storage: store,
		Deps:    EngineDeps{Provider: rec, AgentCfg: config.AgentConfig{MaxTurns: 5}},
	})
	require.NoError(t, err)

	// Send a request with a different model in the body.
	body := strings.NewReader(`{"user_message":"hi","model":"openai/gpt-4o"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/conversation/messages", body)
	recoder := httptest.NewRecorder()
	srv.Router().ServeHTTP(recoder, req)

	require.Equal(t, http.StatusAccepted, recoder.Code)

	// Wait for the async engine goroutine to reach the provider.
	require.Eventually(t, func() bool {
		return rec.wasStreamCalled()
	}, 2*time.Second, 10*time.Millisecond)
}

func TestStoredContentToPlainText(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty", "", ""},
		{"json string", `"hello\nworld"`, "hello\nworld"},
		{"json array", `[{"type":"text","text":"hello"},{"type":"text","text":"world"}]`, "hello\nworld"},
		{"legacy plain text", "plain text", "plain text"},
		{"json string with quotes", `"say \"hi\""`, `say "hi"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storedContentToPlainText(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConversationGet_DecodesStoredContent(t *testing.T) {
	store := newTempStore(t)
	// Insert a message with JSON-encoded text content (as stored by agent/storedFromMessage)
	contentJSON, _ := json.Marshal(core.NewTextContent("hello\nworld"))
	require.NoError(t, store.AppendMessage(context.Background(), &storage.StoredMessage{
		Role:    "assistant",
		Content: string(contentJSON),
	}))

	srv, _ := NewServer(&ServerOpts{
		Config: &config.Config{}, Version: "test", Storage: store,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var body ConversationHistoryResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body.Messages, 1)
	assert.Equal(t, "assistant", body.Messages[0].Role)
	assert.Equal(t, "hello\nworld", body.Messages[0].Content)
}
