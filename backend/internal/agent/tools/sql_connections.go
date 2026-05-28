package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SQLConnection describes a single admin-configured database.
type SQLConnection struct {
	DatabaseID       string `json:"database_id"`
	Engine           string `json:"engine"` // "postgresql"|"mysql"|"sqlite"|"sql-server"
	ConnectionString string `json:"connectionString"`
}

// parseSQLConnections parses the raw JSON from the agent_sql_connections setting.
func parseSQLConnections(raw string) ([]SQLConnection, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var out []SQLConnection
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse agent_sql_connections: %w", err)
	}
	return out, nil
}

// findConnection looks up a connection by its database_id.
func findConnection(conns []SQLConnection, dbID string) (*SQLConnection, bool) {
	for i := range conns {
		if conns[i].DatabaseID == dbID {
			return &conns[i], true
		}
	}
	return nil, false
}
