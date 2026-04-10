// storage/sqlite/sqlite_test.go
package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nousresearch/hermes-agent/storage"
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

func TestMigrateIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	// Running migrate twice must not error.
	err := store.Migrate()
	assert.NoError(t, err)
}

func TestMigrateCreatesRequiredTables(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Query sqlite_master to confirm tables exist.
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
	assert.True(t, tables["sessions"], "sessions table should exist")
	assert.True(t, tables["messages"], "messages table should exist")
}

func TestCreateAndGetSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	session := &storage.Session{
		ID:        "sess-001",
		Source:    "cli",
		UserID:    "user-1",
		Model:     "claude-opus-4-6",
		StartedAt: now,
		Title:     "my session",
	}

	err := store.CreateSession(ctx, session)
	require.NoError(t, err)

	got, err := store.GetSession(ctx, "sess-001")
	require.NoError(t, err)
	assert.Equal(t, "sess-001", got.ID)
	assert.Equal(t, "cli", got.Source)
	assert.Equal(t, "user-1", got.UserID)
	assert.Equal(t, "claude-opus-4-6", got.Model)
	assert.Equal(t, "my session", got.Title)
	assert.WithinDuration(t, now, got.StartedAt, time.Second)
}

func TestGetSessionNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetSession(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestUpdateSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "sess-002", Source: "cli", Model: "test", StartedAt: now,
	}))

	end := now.Add(time.Minute)
	err := store.UpdateSession(ctx, "sess-002", &storage.SessionUpdate{
		EndedAt:   &end,
		EndReason: "user_exit",
		Title:     "done",
	})
	require.NoError(t, err)

	got, err := store.GetSession(ctx, "sess-002")
	require.NoError(t, err)
	require.NotNil(t, got.EndedAt)
	assert.Equal(t, "user_exit", got.EndReason)
	assert.Equal(t, "done", got.Title)
}

func TestAddAndGetMessages(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "sess-msg-1", Source: "cli", Model: "test", StartedAt: now,
	}))

	err := store.AddMessage(ctx, "sess-msg-1", &storage.StoredMessage{
		SessionID: "sess-msg-1",
		Role:      "user",
		Content:   `"hello"`,
		Timestamp: now,
	})
	require.NoError(t, err)

	err = store.AddMessage(ctx, "sess-msg-1", &storage.StoredMessage{
		SessionID: "sess-msg-1",
		Role:      "assistant",
		Content:   `"hi there"`,
		Timestamp: now.Add(time.Second),
	})
	require.NoError(t, err)

	msgs, err := store.GetMessages(ctx, "sess-msg-1", 10, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
}

func TestSearchMessagesFTS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "sess-fts-1", Source: "cli", Model: "test", StartedAt: now,
	}))

	require.NoError(t, store.AddMessage(ctx, "sess-fts-1", &storage.StoredMessage{
		SessionID: "sess-fts-1", Role: "user",
		Content: "the quick brown fox jumps", Timestamp: now,
	}))
	require.NoError(t, store.AddMessage(ctx, "sess-fts-1", &storage.StoredMessage{
		SessionID: "sess-fts-1", Role: "assistant",
		Content: "lazy dogs sleep", Timestamp: now.Add(time.Second),
	}))

	results, err := store.SearchMessages(ctx, "fox", &storage.SearchOptions{Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Message.Content, "fox")
}

func TestWithTxCommitsOnSuccess(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.WithTx(ctx, func(tx storage.Tx) error {
		return tx.CreateSession(ctx, &storage.Session{
			ID:        "tx-commit",
			Source:    "cli",
			Model:     "test",
			StartedAt: time.Now().UTC(),
		})
	})
	require.NoError(t, err)

	sess, err := store.GetSession(ctx, "tx-commit")
	require.NoError(t, err)
	assert.Equal(t, "tx-commit", sess.ID)
}

func TestWithTxRollsBackOnError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	wantErr := errors.New("rollback me")
	err := store.WithTx(ctx, func(tx storage.Tx) error {
		if err := tx.CreateSession(ctx, &storage.Session{
			ID: "tx-rollback", Source: "cli", Model: "test", StartedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
		return wantErr
	})
	assert.ErrorIs(t, err, wantErr)

	_, err = store.GetSession(ctx, "tx-rollback")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestWithTxRollsBackOnPanic(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	assert.Panics(t, func() {
		_ = store.WithTx(ctx, func(tx storage.Tx) error {
			_ = tx.CreateSession(ctx, &storage.Session{
				ID: "tx-panic", Source: "cli", Model: "test", StartedAt: time.Now().UTC(),
			})
			panic("boom")
		})
	})

	_, err := store.GetSession(ctx, "tx-panic")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestUpdateUsageReturnsNotFoundForMissingSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UpdateUsage(ctx, "nonexistent-session", &storage.UsageUpdate{
		InputTokens: 10,
	})
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
