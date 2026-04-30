package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

func TestFeedback(t *testing.T) {
	store := newTempStore(t)
	err := store.AppendMessage(context.Background(), &storage.StoredMessage{
		ID:        1,
		Role:      "assistant",
		Content:   "{}",
		Timestamp: time.Now(),
	})
	require.NoError(t, err)

	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Storage: store,
		Version: "test",
		Streams: NewMemoryStreamHub(),
	})
	require.NoError(t, err)

	body := strings.NewReader(`{"message_id":1,"score":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", body)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}
