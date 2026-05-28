package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDispatchSQLDriver_SQLite_OpensSuccessfully(t *testing.T) {
	db, err := dispatchSQLDriver("sqlite", ":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())
}

func TestDispatchSQLDriver_SQLite3_Alias(t *testing.T) {
	db, err := dispatchSQLDriver("sqlite3", ":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())
}

func TestDispatchSQLDriver_UnknownEngine_ReturnsError(t *testing.T) {
	_, err := dispatchSQLDriver("oracle", "some://dsn")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported SQL engine")
}

func TestDispatchSQLDriver_Postgresql_Opens(t *testing.T) {
	// Verify driver name is registered: sql.Open succeeds (it does not connect eagerly).
	db, err := dispatchSQLDriver("postgresql", "postgres://invalid")
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())
}

func TestDispatchSQLDriver_MySQL_Opens(t *testing.T) {
	db, err := dispatchSQLDriver("mysql", "user:pass@tcp(localhost:3306)/db")
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())
}

func TestDispatchSQLDriver_MSSQL_Opens(t *testing.T) {
	db, err := dispatchSQLDriver("sql-server", "sqlserver://user:pass@localhost")
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())
}
