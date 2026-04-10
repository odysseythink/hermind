// storage/sqlite/sqlite.go
package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync/atomic"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store is the SQLite-backed implementation of storage.Storage.
// Safe for concurrent use. Uses WAL mode.
type Store struct {
	db         *sql.DB
	path       string
	writeCount atomic.Int64
}

// Open creates or opens a SQLite database at the given path.
// The file is created if it does not exist. WAL mode is enabled.
// Call Migrate() after Open() to apply schema.
func Open(path string) (*Store, error) {
	// Ensure parent directory exists
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		// Caller is responsible for creating the parent directory.
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
