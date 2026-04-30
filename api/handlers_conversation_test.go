package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
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
	require.Len(t, history, 0)
}
