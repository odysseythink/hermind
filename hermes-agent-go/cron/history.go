package cron

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Run is a single cron job execution record.
type Run struct {
	ID        int64
	JobName   string
	StartedAt time.Time
	EndedAt   time.Time
	Status    string // "ok" | "error"
	Error     string
	DurationMS int64
}

// HistoryStore is the persistence interface. Callers pass a SQLite
// handle (*sql.DB) to NewSQLiteHistory for the default implementation.
type HistoryStore interface {
	Record(ctx context.Context, r Run) (int64, error)
	Recent(ctx context.Context, jobName string, limit int) ([]Run, error)
}

// SQLiteHistory is a minimal wrapper around a *sql.DB. It owns the
// cron_runs table and creates it on demand.
type SQLiteHistory struct {
	db *sql.DB
}

// NewSQLiteHistory wraps db and creates the cron_runs table if needed.
func NewSQLiteHistory(db *sql.DB) (*SQLiteHistory, error) {
	if db == nil {
		return nil, errors.New("cron: nil db")
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cron_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_name TEXT NOT NULL,
		started_at REAL NOT NULL,
		ended_at REAL NOT NULL,
		status TEXT NOT NULL,
		error TEXT,
		duration_ms INTEGER NOT NULL
	)`); err != nil {
		return nil, err
	}
	return &SQLiteHistory{db: db}, nil
}

// Record appends one run entry.
func (h *SQLiteHistory) Record(ctx context.Context, r Run) (int64, error) {
	res, err := h.db.ExecContext(ctx, `INSERT INTO cron_runs
		(job_name, started_at, ended_at, status, error, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.JobName,
		float64(r.StartedAt.UnixNano())/1e9,
		float64(r.EndedAt.UnixNano())/1e9,
		r.Status,
		r.Error,
		r.DurationMS,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Recent returns up to limit runs for job_name, newest first.
func (h *SQLiteHistory) Recent(ctx context.Context, jobName string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := h.db.QueryContext(ctx, `SELECT id, job_name, started_at, ended_at, status, error, duration_ms
		FROM cron_runs WHERE job_name = ? ORDER BY started_at DESC LIMIT ?`, jobName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		var startF, endF float64
		var errStr sql.NullString
		if err := rows.Scan(&r.ID, &r.JobName, &startF, &endF, &r.Status, &errStr, &r.DurationMS); err != nil {
			return nil, err
		}
		r.StartedAt = time.Unix(0, int64(startF*1e9)).UTC()
		r.EndedAt = time.Unix(0, int64(endF*1e9)).UTC()
		if errStr.Valid {
			r.Error = errStr.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
