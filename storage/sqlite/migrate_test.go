package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_CreatesSchemaMetaAtV1(t *testing.T) {
	store := newTestStore(t)
	var value string
	err := store.db.QueryRowContext(
		context.Background(),
		`SELECT value FROM schema_meta WHERE key = 'version'`,
	).Scan(&value)
	require.NoError(t, err)
	// Until Task 3 ships, freshly-initialized DBs stay at version 1.
	assert.Equal(t, "1", value)
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
	assert.Equal(t, "1", value)
}
