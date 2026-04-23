// storage/sqlite/sqlite.go
package sqlite

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store is the SQLite-backed implementation of storage.Storage.
// Safe for concurrent use. Uses WAL mode.
type Store struct {
	db   *sql.DB
	path string
}

// Open creates or opens a SQLite database at the given path.
// The file is created if it does not exist. WAL mode is enabled.
// Call Migrate() after Open() to apply schema.
//
// If an existing file has a v1 `sessions` table, it is renamed to
// `<path>.v1-backup` and a fresh v3 DB is created in its place. The
// user's message history is preserved in the backup but is not
// migrated.
func Open(path string) (*Store, error) {
	backedUp, backupPath, err := backupLegacyDBIfNeeded(path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	if backedUp {
		fmt.Fprintln(os.Stderr,
			"hermind: legacy state.db (v1 multi-session) backed up to "+backupPath+".")
		fmt.Fprintln(os.Stderr,
			"  The new schema is single-conversation; your message history has been")
		fmt.Fprintln(os.Stderr,
			"  preserved in the backup but is not migrated.")
	}

	// modernc.org/sqlite uses the name "sqlite" not "sqlite3"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}

	// Apply pragmas: WAL mode for concurrent reads, foreign keys on, 1s busy timeout
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=1000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite: pragma %q: %w", p, err)
		}
	}

	// SQLite only supports one writer at a time. Cap the pool to serialize
	// write transactions and avoid busy_timeout contention under load.
	db.SetMaxOpenConns(1)

	return &Store{db: db, path: path}, nil
}

// Close shuts down the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB. Intended for tests that need to
// issue direct queries; production code should go through storage.Tx.
func (s *Store) DB() *sql.DB { return s.db }
