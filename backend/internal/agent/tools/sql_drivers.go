package tools

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"  // mysql
	_ "github.com/lib/pq"               // postgres
	_ "github.com/mattn/go-sqlite3"     // sqlite
	_ "github.com/microsoft/go-mssqldb" // sql-server
)

// dispatchSQLDriver opens a *sql.DB for the given engine and connection string.
func dispatchSQLDriver(engine, connString string) (*sql.DB, error) {
	switch engine {
	case "postgresql", "postgres":
		return sql.Open("postgres", connString)
	case "mysql":
		return sql.Open("mysql", connString)
	case "sqlite", "sqlite3":
		return sql.Open("sqlite3", connString)
	case "sql-server", "mssql":
		return sql.Open("sqlserver", connString)
	default:
		return nil, fmt.Errorf("unsupported SQL engine: %q", engine)
	}
}
