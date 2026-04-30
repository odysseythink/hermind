package sqlite

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateMessage(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	msg := &storage.StoredMessage{Role: "user", Content: "hello"}
	require.NoError(t, store.AppendMessage(ctx, msg))
	require.NotZero(t, msg.ID)

	err := store.UpdateMessage(ctx, msg.ID, "updated")
	require.NoError(t, err)

	history, err := store.GetHistory(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "updated", history[0].Content)
}

func TestDeleteMessage(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for _, content := range []string{"a", "b", "c"} {
		msg := &storage.StoredMessage{Role: "user", Content: content}
		require.NoError(t, store.AppendMessage(ctx, msg))
	}

	history, _ := store.GetHistory(ctx, 10, 0)
	require.Len(t, history, 3)

	err := store.DeleteMessage(ctx, history[1].ID)
	require.NoError(t, err)

	history, _ = store.GetHistory(ctx, 10, 0)
	require.Len(t, history, 2)
	assert.Equal(t, "a", history[0].Content)
	assert.Equal(t, "c", history[1].Content)
}

func TestDeleteMessagesAfter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for _, content := range []string{"a", "b", "c"} {
		msg := &storage.StoredMessage{Role: "user", Content: content}
		require.NoError(t, store.AppendMessage(ctx, msg))
	}

	history, _ := store.GetHistory(ctx, 10, 0)
	require.Len(t, history, 3)

	err := store.DeleteMessagesAfter(ctx, history[0].ID)
	require.NoError(t, err)

	history, _ = store.GetHistory(ctx, 10, 0)
	require.Len(t, history, 1)
	assert.Equal(t, "a", history[0].Content)
}
