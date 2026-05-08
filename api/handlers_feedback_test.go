package api

import (
	"context"
	"fmt"
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
	msg := &storage.StoredMessage{
		Role:      "assistant",
		Content:   "{}",
		Timestamp: time.Now(),
	}
	err := store.AppendMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotZero(t, msg.ID)

	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Storage: store,
		Version: "test",
		Streams: NewMemoryStreamHub(),
	})
	require.NoError(t, err)

	body := strings.NewReader(fmt.Sprintf(`{"message_id":%d,"score":1}`, msg.ID))
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", body)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}
