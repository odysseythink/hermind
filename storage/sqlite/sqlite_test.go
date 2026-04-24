// storage/sqlite/sqlite_test.go
package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpenCreatesDatabaseFile(t *testing.T) {
	store := newTestStore(t)
	assert.NotNil(t, store)
}

func TestMigrateCreatesV4Tables(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rows, err := store.db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables[name] = true
	}
	assert.True(t, tables["messages"], "messages table should exist")
	assert.True(t, tables["conversation_state"], "conversation_state table should exist")
	assert.True(t, tables["memories"], "memories table should exist")
	assert.False(t, tables["sessions"], "sessions table must not exist in v4")
}

func TestAppendAndGetHistory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
		Role:      "user",
		Content:   `"hello"`,
		Timestamp: now,
	}))
	require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
		Role:      "assistant",
		Content:   `"hi there"`,
		Timestamp: now.Add(time.Second),
	}))

	msgs, err := store.GetHistory(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
}

func TestSearchMessagesFTS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
		Role:    "user",
		Content: "the quick brown fox jumps",
		Timestamp: now,
	}))
	require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
		Role:    "assistant",
		Content: "lazy dogs sleep",
		Timestamp: now.Add(time.Second),
	}))

	results, err := store.SearchMessages(ctx, "fox", &storage.SearchOptions{Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Message.Content, "fox")
}

func TestUpdateSystemPromptCache(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.UpdateSystemPromptCache(ctx, "cached prompt"))

	var got string
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT system_prompt_cache FROM conversation_state WHERE id=1`).Scan(&got))
	assert.Equal(t, "cached prompt", got)
}

func TestUpdateUsageAggregates(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.UpdateUsage(ctx, &storage.UsageUpdate{
		InputTokens: 100, OutputTokens: 50, CostUSD: 0.25,
	}))
	require.NoError(t, store.UpdateUsage(ctx, &storage.UsageUpdate{
		InputTokens: 10, OutputTokens: 5,
	}))

	var inTok, outTok int
	var cost float64
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT total_input_tokens, total_output_tokens, total_cost_usd FROM conversation_state WHERE id=1`,
	).Scan(&inTok, &outTok, &cost))
	assert.Equal(t, 110, inTok)
	assert.Equal(t, 55, outTok)
	assert.InDelta(t, 0.25, cost, 0.0001)
}

func TestWithTxCommitsOnSuccess(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.WithTx(ctx, func(tx storage.Tx) error {
		return tx.AppendMessage(ctx, &storage.StoredMessage{
			Role: "user", Content: `"tx commit"`, Timestamp: time.Now().UTC(),
		})
	})
	require.NoError(t, err)

	msgs, err := store.GetHistory(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "tx commit")
}

func TestWithTxRollsBackOnError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	wantErr := errors.New("rollback me")
	err := store.WithTx(ctx, func(tx storage.Tx) error {
		if err := tx.AppendMessage(ctx, &storage.StoredMessage{
			Role: "user", Content: `"should roll back"`, Timestamp: time.Now().UTC(),
		}); err != nil {
			return err
		}
		return wantErr
	})
	assert.ErrorIs(t, err, wantErr)

	msgs, err := store.GetHistory(ctx, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestWithTxRollsBackOnPanic(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	assert.Panics(t, func() {
		_ = store.WithTx(ctx, func(tx storage.Tx) error {
			_ = tx.AppendMessage(ctx, &storage.StoredMessage{
				Role: "user", Content: `"panic"`, Timestamp: time.Now().UTC(),
			})
			panic("boom")
		})
	})

	msgs, err := store.GetHistory(ctx, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}
