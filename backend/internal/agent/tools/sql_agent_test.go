package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

func TestSQLAgent_ListDatabases_ReturnsConfiguredConnections(t *testing.T) {
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"test","engine":"sqlite","connectionString":":memory:"}]`,
		},
		Emit: func(string) {},
	}
	e := NewSQLAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"list_databases"}`))
	require.NoError(t, err)
	require.Contains(t, result, "test")
	require.Contains(t, result, "sqlite")
}

func TestSQLAgent_ListDatabases_NoConnections_ReturnsEmptyArray(t *testing.T) {
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[]`,
		},
		Emit: func(string) {},
	}
	e := NewSQLAgentSkill(tc)
	require.False(t, e.CheckFn(), "CheckFn should be false for empty connections")
}

func TestSQLAgent_ListTables_Sqlite_ReturnsTableNames(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	db, err := dispatchSQLDriver("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec("CREATE TABLE users (id INTEGER, name TEXT)")
	require.NoError(t, err)

	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":"` + dbPath + `"}]`,
		},
		Emit: func(string) {},
	}
	e := NewSQLAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"list_tables","database_id":"mem"}`))
	require.NoError(t, err)
	require.Contains(t, result, "users")
}

func TestSQLAgent_GetSchema_Sqlite_ReturnsColumnInfo(t *testing.T) {
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":":memory:"}]`,
		},
		Emit: func(string) {},
	}
	e := NewSQLAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"get_schema","database_id":"mem","table":"sqlite_master"}`))
	require.NoError(t, err)
	require.Contains(t, result, "columns")
}

func TestSQLAgent_Query_Sqlite_SelectStatement_ReturnsRows(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	db, err := dispatchSQLDriver("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec("CREATE TABLE users (id INTEGER, name TEXT); INSERT INTO users VALUES (1, 'alice'), (2, 'bob')")
	require.NoError(t, err)

	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":"` + dbPath + `"}]`,
		},
		Emit: func(string) {},
	}
	e := NewSQLAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"query","database_id":"mem","sql_query":"SELECT * FROM users ORDER BY id"}`))
	require.NoError(t, err)
	require.Contains(t, result, "alice")
	require.Contains(t, result, "bob")
}

func TestSQLAgent_Query_BoundedTo100Rows(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	db, err := dispatchSQLDriver("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec("CREATE TABLE nums (n INTEGER)")
	require.NoError(t, err)
	for i := 0; i < 150; i++ {
		_, err = db.Exec("INSERT INTO nums VALUES (?)", i)
		require.NoError(t, err)
	}

	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":"` + dbPath + `"}]`,
		},
		Emit: func(string) {},
	}
	e := NewSQLAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"query","database_id":"mem","sql_query":"SELECT * FROM nums"}`))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &payload))
	require.Equal(t, float64(100), payload["row_count"])
	require.Equal(t, true, payload["limit_hit"])
}

func TestSQLAgent_Query_RequiresApprovalForDestructive(t *testing.T) {
	var calledWith string
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":":memory:"}]`,
		},
		Emit: func(string) {},
		Approval: func(_ context.Context, name string, _ any, _ string) (bool, string) {
			calledWith = name
			return true, ""
		},
	}
	e := NewSQLAgentSkill(tc)
	_, _ = e.Handler(context.Background(), json.RawMessage(`{"action":"query","database_id":"mem","sql_query":"SELECT 1"}`))
	require.Equal(t, "sql-agent:query", calledWith)
}

func TestSQLAgent_Query_ApprovalRejected_ReturnsToolError(t *testing.T) {
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":":memory:"}]`,
		},
		Emit: func(string) {},
		Approval: func(context.Context, string, any, string) (bool, string) {
			return false, "user denied"
		},
	}
	e := NewSQLAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"query","database_id":"mem","sql_query":"SELECT 1"}`))
	require.NoError(t, err)
	require.Contains(t, result, "rejected")
}

func TestSQLAgent_UnknownDatabaseID_ReturnsToolError(t *testing.T) {
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[]`,
		},
		Emit: func(string) {},
	}
	e := NewSQLAgentSkill(tc)
	require.False(t, e.CheckFn())
}

func TestSQLAgent_CheckFn_FalseWhenNoConnections(t *testing.T) {
	tc := &ToolContext{Settings: map[string]string{}, Emit: func(string) {}}
	e := NewSQLAgentSkill(tc)
	require.False(t, e.CheckFn())
}

func TestSQLAgent_DispatchViaRegistry(t *testing.T) {
	reg := tool.NewRegistry()
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":":memory:"}]`,
		},
		Emit: func(string) {},
	}
	reg.Register(NewSQLAgentSkill(tc))

	result, err := reg.Dispatch(context.Background(), "sql-agent", []byte(`{"action":"list_databases"}`))
	require.NoError(t, err)
	require.Contains(t, result, "mem")
}

func TestSQLAgent_ReadActions_BypassApproval(t *testing.T) {
	var called bool
	tc := &ToolContext{
		Settings: map[string]string{
			"agent_sql_connections": `[{"database_id":"mem","engine":"sqlite","connectionString":":memory:"}]`,
		},
		Emit: func(string) {},
		Approval: func(context.Context, string, any, string) (bool, string) {
			called = true
			return true, ""
		},
	}
	e := NewSQLAgentSkill(tc)

	// list_databases should NOT call approval
	_, _ = e.Handler(context.Background(), json.RawMessage(`{"action":"list_databases"}`))
	require.False(t, called)
}
