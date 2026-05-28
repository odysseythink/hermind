package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

const sqlMaxRows = 100

func NewSQLAgentSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "sql-agent",
		Toolset:        "sql",
		Description:    "Inspect and query admin-configured SQL databases. Actions: list_databases, list_tables, get_schema, query.",
		Emoji:          "🗄",
		MaxResultChars: 16 * 1024,
		CheckFn: func() bool {
			if tc.Settings == nil {
				return false
			}
			v := strings.TrimSpace(tc.Settings["agent_sql_connections"])
			return v != "" && v != "null" && v != "[]"
		},
		Schema: core.ToolDefinition{
			Name:        "sql-agent",
			Description: "Inspect or query SQL databases",
			Parameters:  sqlAgentSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Action     string `json:"action"`
				DatabaseID string `json:"database_id,omitempty"`
				SQLQuery   string `json:"sql_query,omitempty"`
				Table      string `json:"table,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}

			conns, err := parseSQLConnections(tc.Settings["agent_sql_connections"])
			if err != nil {
				return tool.Error(err.Error()), nil
			}

			switch args.Action {
			case "list_databases":
				tc.Emit("Listing configured SQL databases")
				out := make([]map[string]any, 0, len(conns))
				for _, c := range conns {
					out = append(out, map[string]any{"database_id": c.DatabaseID, "engine": c.Engine})
				}
				return tool.Result(map[string]any{"databases": out}), nil

			case "list_tables":
				conn, ok := findConnection(conns, args.DatabaseID)
				if !ok {
					return tool.Error("unknown database_id: " + args.DatabaseID), nil
				}
				tc.Emit("Listing tables in " + args.DatabaseID)
				db, err := dispatchSQLDriver(conn.Engine, conn.ConnectionString)
				if err != nil {
					return tool.Error("connect: " + err.Error()), nil
				}
				defer db.Close()
				tables, err := listTables(ctx, db, conn.Engine)
				if err != nil {
					return tool.Error("list tables: " + err.Error()), nil
				}
				return tool.Result(map[string]any{"tables": tables}), nil

			case "get_schema":
				conn, ok := findConnection(conns, args.DatabaseID)
				if !ok {
					return tool.Error("unknown database_id: " + args.DatabaseID), nil
				}
				if args.Table == "" {
					return tool.Error("table is required"), nil
				}
				tc.Emit(fmt.Sprintf("Inspecting %s.%s", args.DatabaseID, args.Table))
				db, err := dispatchSQLDriver(conn.Engine, conn.ConnectionString)
				if err != nil {
					return tool.Error("connect: " + err.Error()), nil
				}
				defer db.Close()
				cols, err := tableSchema(ctx, db, conn.Engine, args.Table)
				if err != nil {
					return tool.Error("schema: " + err.Error()), nil
				}
				return tool.Result(map[string]any{"table": args.Table, "columns": cols}), nil

			case "query":
				conn, ok := findConnection(conns, args.DatabaseID)
				if !ok {
					return tool.Error("unknown database_id: " + args.DatabaseID), nil
				}
				if args.SQLQuery == "" {
					return tool.Error("sql_query is required"), nil
				}

				// Approval gate (destructive action)
				if tc.Approval != nil {
					desc := fmt.Sprintf("Run SQL on %s: %s", args.DatabaseID, truncate(args.SQLQuery, 200))
					approved, reason := tc.Approval(ctx, "sql-agent:query", args, desc)
					if !approved {
						return tool.Error("rejected: " + reason), nil
					}
				}

				tc.Emit("Running SQL query on " + args.DatabaseID)
				db, err := dispatchSQLDriver(conn.Engine, conn.ConnectionString)
				if err != nil {
					return tool.Error("connect: " + err.Error()), nil
				}
				defer db.Close()
				rows, err := runQuery(ctx, db, args.SQLQuery, sqlMaxRows)
				if err != nil {
					return tool.Error("query: " + err.Error()), nil
				}
				return tool.Result(map[string]any{
					"rows":      rows,
					"row_count": len(rows),
					"limit_hit": len(rows) == sqlMaxRows,
				}), nil

			default:
				return tool.Error("unknown action: " + args.Action), nil
			}
		},
	}
}

func sqlAgentSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"action":      {"type": "string", "enum": ["list_databases", "list_tables", "get_schema", "query"]},
			"database_id": {"type": "string"},
			"sql_query":   {"type": "string"},
			"table":       {"type": "string"}
		},
		"required": ["action"]
	}`))
}

// listTables returns table names for the given engine.
func listTables(ctx context.Context, db *sql.DB, engine string) ([]string, error) {
	var q string
	switch engine {
	case "postgresql", "postgres":
		q = "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name"
	case "mysql":
		q = "SELECT table_name FROM information_schema.tables WHERE table_schema=DATABASE() ORDER BY table_name"
	case "sqlite", "sqlite3":
		q = "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
	case "sql-server", "mssql":
		q = "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE' ORDER BY TABLE_NAME"
	default:
		return nil, fmt.Errorf("unsupported engine: %s", engine)
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// tableSchema returns column metadata for the named table.
func tableSchema(ctx context.Context, db *sql.DB, engine, table string) ([]map[string]any, error) {
	var q string
	switch engine {
	case "postgresql", "postgres":
		q = "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name=$1 ORDER BY ordinal_position"
	case "mysql":
		q = "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name=? AND table_schema=DATABASE() ORDER BY ordinal_position"
	case "sqlite", "sqlite3":
		return sqliteTableSchema(ctx, db, table)
	case "sql-server", "mssql":
		q = "SELECT column_name, data_type, is_nullable FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME=@p1"
	default:
		return nil, fmt.Errorf("unsupported engine: %s", engine)
	}
	rows, err := db.QueryContext(ctx, q, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var name, dtype, nullable string
		if err := rows.Scan(&name, &dtype, &nullable); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": name, "type": dtype, "nullable": nullable == "YES"})
	}
	return out, rows.Err()
}

// sqliteTableSchema uses PRAGMA table_info(<table>).
func sqliteTableSchema(ctx context.Context, db *sql.DB, table string) ([]map[string]any, error) {
	for _, r := range table {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return nil, fmt.Errorf("invalid table name: %q", table)
		}
	}
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"name":        name,
			"type":        ctype,
			"nullable":    notnull == 0,
			"primary_key": pk == 1,
		})
	}
	return out, rows.Err()
}

// runQuery executes a query and returns up to maxRows results.
func runQuery(ctx context.Context, db *sql.DB, query string, maxRows int) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() && len(out) < maxRows {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = values[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
