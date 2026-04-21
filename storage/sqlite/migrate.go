// storage/sqlite/migrate.go
package sqlite

import (
	"fmt"
	"strconv"
	"strings"
)

// schemaSQL is the full initial schema. Designed to match the Python hermes
// state.db layout for compatibility. schema_meta tracks the applied version
// so incremental migrations beyond v1 can run idempotently.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL DEFAULT 'cli',
    user_id TEXT DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    model_config TEXT DEFAULT '{}',
    system_prompt TEXT DEFAULT '',
    parent_session_id TEXT DEFAULT '',
    started_at REAL NOT NULL,
    ended_at REAL,
    end_reason TEXT DEFAULT '',
    message_count INTEGER NOT NULL DEFAULT 0,
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    billing_provider TEXT DEFAULT '',
    billing_base_url TEXT DEFAULT '',
    estimated_cost_usd REAL NOT NULL DEFAULT 0,
    actual_cost_usd REAL NOT NULL DEFAULT 0,
    cost_status TEXT DEFAULT '',
    title TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions(source);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON sessions(started_at);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    tool_calls TEXT DEFAULT '',
    tool_name TEXT DEFAULT '',
    timestamp REAL NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    finish_reason TEXT DEFAULT '',
    reasoning TEXT DEFAULT '',
    reasoning_details TEXT DEFAULT '',
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    content='messages',
    content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_update AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    user_id TEXT DEFAULT '',
    content TEXT NOT NULL,
    category TEXT DEFAULT '',
    tags TEXT DEFAULT '',
    metadata TEXT DEFAULT '{}',
    created_at REAL NOT NULL,
    updated_at REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    content,
    content='memories',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS memories_fts_insert AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_delete AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_update AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
    INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TABLE IF NOT EXISTS schema_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '1');
`

// currentSchemaVersion is the highest version this binary knows about. Any
// stored version less than this triggers incremental migration steps in
// Migrate(). When adding a new step, bump this constant AND add the matching
// case in applyVersion.
const currentSchemaVersion = 2

// Migrate applies the base schema, then runs any versioned migration steps
// up to currentSchemaVersion. Idempotent: safe to call on an up-to-date DB.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("sqlite: migrate base: %w", err)
	}
	for {
		v, err := s.schemaVersion()
		if err != nil {
			return err
		}
		if v >= currentSchemaVersion {
			return nil
		}
		if err := s.applyVersion(v + 1); err != nil {
			return fmt.Errorf("sqlite: migrate v%d: %w", v+1, err)
		}
	}
}

func (s *Store) schemaVersion() (int, error) {
	var raw string
	err := s.db.QueryRow(
		`SELECT value FROM schema_meta WHERE key = 'version'`,
	).Scan(&raw)
	if err != nil {
		return 0, fmt.Errorf("sqlite: read schema version: %w", err)
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("sqlite: parse schema version %q: %w", raw, err)
	}
	return v, nil
}

// applyVersion dispatches to the step that bumps the DB from v-1 to v.
// Runs in a single transaction so partial failures leave the version unchanged.
func (s *Store) applyVersion(v int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch v {
	case 2:
		if _, err := tx.Exec(`DELETE FROM messages`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM sessions`); err != nil {
			return err
		}
	default:
		return fmt.Errorf("no migration step for v%d", v)
	}

	if _, err := tx.Exec(
		`UPDATE schema_meta SET value = ? WHERE key = 'version'`,
		fmt.Sprintf("%d", v),
	); err != nil {
		return err
	}
	return tx.Commit()
}
