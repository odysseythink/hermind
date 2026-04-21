package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/storage"
)

func TestMigrate_CreatesSchemaMetaAtCurrentVersion(t *testing.T) {
	store := newTestStore(t)
	var value string
	err := store.db.QueryRowContext(
		context.Background(),
		`SELECT value FROM schema_meta WHERE key = 'version'`,
	).Scan(&value)
	require.NoError(t, err)
	// A freshly-initialized DB is migrated all the way up to currentSchemaVersion.
	assert.Equal(t, "2", value)
}

func TestMigrate_Idempotent(t *testing.T) {
	store := newTestStore(t)
	// Migrate a second time; must not fail, must not bump version unnecessarily.
	require.NoError(t, store.Migrate())
	var value string
	require.NoError(t, store.db.QueryRowContext(
		context.Background(),
		`SELECT value FROM schema_meta WHERE key = 'version'`,
	).Scan(&value))
	assert.Equal(t, "2", value)
}

func TestMigrate_V2_WipesSessionsAndMessages(t *testing.T) {
	store := newTestStore(t) // migrated to v1
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "s1", Source: "cli", Model: "m", StartedAt: now,
	}))
	require.NoError(t, store.AddMessage(ctx, "s1", &storage.StoredMessage{
		Role: "user", Content: `"hi"`, Timestamp: now,
	}))
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "mem1", Content: "kept across migrations", CreatedAt: now, UpdatedAt: now,
	}))

	// Force the runner back to v1 so Migrate() advances to v2.
	_, err := store.db.ExecContext(ctx,
		`UPDATE schema_meta SET value = '1' WHERE key = 'version'`)
	require.NoError(t, err)

	require.NoError(t, store.Migrate())

	var sessionCount, messageCount, memoryCount int
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions`).Scan(&sessionCount))
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages`).Scan(&messageCount))
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories`).Scan(&memoryCount))

	assert.Equal(t, 0, sessionCount, "v2 should wipe sessions")
	assert.Equal(t, 0, messageCount, "v2 should wipe messages")
	assert.Equal(t, 1, memoryCount, "v2 must NOT touch memories")

	var version string
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT value FROM schema_meta WHERE key = 'version'`).Scan(&version))
	assert.Equal(t, "2", version)
}

func TestMigrate_V2_Idempotent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	// First Migrate() in newTestStore already ran. Populate, then re-Migrate.
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "survivor", Source: "cli", Model: "m", StartedAt: time.Now().UTC(),
	}))
	require.NoError(t, store.Migrate())

	var count int
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions`).Scan(&count))
	// Already at v2; second Migrate() is a no-op; row survives.
	assert.Equal(t, 1, count)
}
