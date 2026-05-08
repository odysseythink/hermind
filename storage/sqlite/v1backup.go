package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// backupLegacyDBIfNeeded renames an existing state.db to state.db.v1-backup
// if it has a v1 `sessions` table. Returns (renamed, backupPath, error).
// If the source file doesn't exist or doesn't have a sessions table, it
// is a no-op.
//
// On collision with an existing .v1-backup, a unix-ms suffix is appended
// so no data is ever overwritten.
func backupLegacyDBIfNeeded(path string) (bool, string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", err
	}

	hasSessions, err := probeSessionsTable(path)
	if err != nil {
		return false, "", err
	}
	if !hasSessions {
		return false, "", nil
	}

	backup := path + ".v1-backup"
	if _, err := os.Stat(backup); err == nil {
		backup = fmt.Sprintf("%s.%d", backup, time.Now().UnixMilli())
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, "", err
	}

	if err := os.Rename(path, backup); err != nil {
		return false, "", fmt.Errorf("sqlite: backup legacy db: %w", err)
	}
	return true, backup, nil
}

// probeSessionsTable opens the DB read-only and checks for a `sessions`
// table.
func probeSessionsTable(path string) (bool, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return false, fmt.Errorf("sqlite: probe open: %w", err)
	}
	defer db.Close()

	row := db.QueryRow(`SELECT 1 FROM sqlite_master WHERE type='table' AND name='sessions' LIMIT 1`)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("sqlite: probe scan: %w", err)
	}
	return one == 1, nil
}
