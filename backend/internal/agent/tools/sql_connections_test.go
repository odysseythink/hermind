package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSQLConnections_Empty_ReturnsNil(t *testing.T) {
	conns, err := parseSQLConnections("")
	require.NoError(t, err)
	require.Nil(t, conns)
}

func TestParseSQLConnections_Null_ReturnsNil(t *testing.T) {
	conns, err := parseSQLConnections("null")
	require.NoError(t, err)
	require.Nil(t, conns)
}

func TestParseSQLConnections_MalformedJSON_ReturnsError(t *testing.T) {
	_, err := parseSQLConnections("not json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse agent_sql_connections")
}

func TestParseSQLConnections_ValidJSON_ReturnsParsed(t *testing.T) {
	raw := `[
		{"database_id":"prod","engine":"postgresql","connectionString":"postgres://u:p@h/db"},
		{"database_id":"local","engine":"sqlite","connectionString":"file:./local.db"}
	]`
	conns, err := parseSQLConnections(raw)
	require.NoError(t, err)
	require.Len(t, conns, 2)
	require.Equal(t, "prod", conns[0].DatabaseID)
	require.Equal(t, "postgresql", conns[0].Engine)
	require.Equal(t, "local", conns[1].DatabaseID)
	require.Equal(t, "sqlite", conns[1].Engine)
}

func TestFindConnection_Found(t *testing.T) {
	conns := []SQLConnection{
		{DatabaseID: "a", Engine: "sqlite"},
		{DatabaseID: "b", Engine: "postgresql"},
	}
	c, ok := findConnection(conns, "b")
	require.True(t, ok)
	require.Equal(t, "postgresql", c.Engine)
}

func TestFindConnection_NotFound(t *testing.T) {
	conns := []SQLConnection{{DatabaseID: "a", Engine: "sqlite"}}
	_, ok := findConnection(conns, "missing")
	require.False(t, ok)
}
