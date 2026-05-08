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
INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '3');
`

// currentSchemaVersion: v4 added MemType+Vector; v5 added supersession
// lifecycle columns (status, superseded_by); v6 adds reinforcement tracking;
// v7 adds memory_events table for event-driven memory consolidation;
// v8 adds skills_generation table + memories.reinforced_at_seq column;
// v9 adds feedback and attachments tables.
const currentSchemaVersion = 9

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
		// ALTER TABLE is tolerant of fresh databases where schemaSQL already
		// created the column — SQLite returns "duplicate column name" in that case.
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN mem_type TEXT NOT NULL DEFAULT ''`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v4 add mem_type: %w", err)
			}
		}
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN vector BLOB`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v4 add vector: %w", err)
			}
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_mem_type ON memories(mem_type)`); err != nil {
			return fmt.Errorf("v4 add index: %w", err)
		}
	case 5:
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN status TEXT NOT NULL DEFAULT 'active'`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v5 add status: %w", err)
			}
		}
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN superseded_by TEXT NOT NULL DEFAULT ''`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v5 add superseded_by: %w", err)
			}
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_status ON memories(status)`); err != nil {
			return fmt.Errorf("v5 add status index: %w", err)
		}
	case 6:
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN reinforcement_count INTEGER NOT NULL DEFAULT 0`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v6 add reinforcement_count: %w", err)
			}
		}
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN neglect_count INTEGER NOT NULL DEFAULT 0`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v6 add neglect_count: %w", err)
			}
		}
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN last_used_at REAL NOT NULL DEFAULT 0`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v6 add last_used_at: %w", err)
			}
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_last_used ON memories(last_used_at)`); err != nil {
			return fmt.Errorf("v6 add last_used_at index: %w", err)
		}
	case 7:
		if _, err := tx.Exec(`
        CREATE TABLE IF NOT EXISTS memory_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ts REAL NOT NULL,
            kind TEXT NOT NULL,
            data TEXT NOT NULL DEFAULT '{}'
        )`); err != nil {
			return fmt.Errorf("v7 create memory_events: %w", err)
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memory_events_ts ON memory_events(ts DESC)`); err != nil {
			return fmt.Errorf("v7 memory_events ts index: %w", err)
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_memory_events_kind ON memory_events(kind)`); err != nil {
			return fmt.Errorf("v7 memory_events kind index: %w", err)
		}
	case 8:
		if _, err := tx.Exec(`
	        CREATE TABLE IF NOT EXISTS skills_generation (
	            id         INTEGER PRIMARY KEY CHECK (id = 1),
	            hash       TEXT    NOT NULL DEFAULT '',
	            seq        INTEGER NOT NULL DEFAULT 0,
	            updated_at REAL    NOT NULL DEFAULT 0
	        )`); err != nil {
			return fmt.Errorf("v8 create skills_generation: %w", err)
		}
		if _, err := tx.Exec(`
	        INSERT OR IGNORE INTO skills_generation (id, hash, seq, updated_at)
	        VALUES (1, '', 0, 0)`); err != nil {
			return fmt.Errorf("v8 seed skills_generation row: %w", err)
		}
		if _, err := tx.Exec(`ALTER TABLE memories ADD COLUMN reinforced_at_seq INTEGER NOT NULL DEFAULT 0`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("v8 add reinforced_at_seq: %w", err)
			}
		}
	case 9:
		if _, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS feedback (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				message_id INTEGER NOT NULL UNIQUE,
				score INTEGER NOT NULL,
				created_at TEXT DEFAULT (datetime('now')),
				FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
			);
			CREATE TABLE IF NOT EXISTS attachments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				message_id INTEGER NOT NULL,
				name TEXT NOT NULL,
				type TEXT NOT NULL,
				url TEXT NOT NULL,
				size INTEGER NOT NULL DEFAULT 0,
				created_at TEXT DEFAULT (datetime('now')),
				FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
			);
		`); err != nil {
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
