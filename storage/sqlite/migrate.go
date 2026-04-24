// storage/sqlite/migrate.go
package sqlite

import (
	"fmt"
	"strconv"
	"strings"
)

// schemaSQL is the v4 schema. messages are instance-scoped (no
// session_id); conversation_state is a singleton row that tracks
// per-instance totals; memories now include mem_type and vector fields.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    tool_calls TEXT DEFAULT '',
    tool_name TEXT DEFAULT '',
    timestamp REAL NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    finish_reason TEXT DEFAULT '',
    reasoning TEXT DEFAULT '',
    reasoning_details TEXT DEFAULT ''
);

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

CREATE TABLE IF NOT EXISTS conversation_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    system_prompt_cache TEXT DEFAULT '',
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens INTEGER DEFAULT 0,
    total_cache_write_tokens INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0,
    updated_at REAL NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO conversation_state
    (id, updated_at) VALUES (1, 0);

CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    user_id TEXT DEFAULT '',
    content TEXT NOT NULL,
    category TEXT DEFAULT '',
    tags TEXT DEFAULT '',
    metadata TEXT DEFAULT '{}',
    created_at REAL NOT NULL,
    updated_at REAL NOT NULL,
    mem_type TEXT NOT NULL DEFAULT '',
    vector BLOB
);
CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);
CREATE INDEX IF NOT EXISTS idx_memories_mem_type ON memories(mem_type);

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
INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '4');
`

// currentSchemaVersion is the v4 single-conversation schema with MemType and Vector.
// v1 and v2 DBs are detected by backupLegacyDBIfNeeded() at Open() time and
// renamed out of the way, so no in-place migration code is needed.
const currentSchemaVersion = 4

// Migrate applies the base schema. Idempotent. Legacy v1/v2 DBs are
// never reached here — they are backed up before Migrate() runs.
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

// applyVersion applies incremental schema migrations.
func (s *Store) applyVersion(v int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch v {
	case 3:
		// no-op: v3 IS the initial schema emitted by schemaSQL
	case 4:
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN mem_type TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("v4 add mem_type: %w", err)
		}
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN vector BLOB`); err != nil {
			return fmt.Errorf("v4 add vector: %w", err)
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_mem_type ON memories(mem_type)`); err != nil {
			return fmt.Errorf("v4 add index: %w", err)
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
