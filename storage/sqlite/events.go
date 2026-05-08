package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// AppendMemoryEvent implements storage.Storage.
func (s *Store) AppendMemoryEvent(ctx context.Context, ts time.Time, kind string, data []byte) error {
	if len(data) == 0 {
		data = []byte("{}")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memory_events (ts, kind, data) VALUES (?, ?, ?)`,
		toEpoch(ts), kind, string(data))
	if err != nil {
		return fmt.Errorf("sqlite: append memory_event: %w", err)
	}
	return nil
}

// ListMemoryEvents implements storage.Storage.
func (s *Store) ListMemoryEvents(ctx context.Context, limit, offset int, kinds []string) ([]*storage.MemoryEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	var (
		rows *sql.Rows
		err  error
	)
	if len(kinds) == 0 {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, ts, kind, data FROM memory_events ORDER BY ts DESC LIMIT ? OFFSET ?`,
			limit, offset)
	} else {
		placeholders := strings.Repeat("?,", len(kinds))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, 0, len(kinds)+2)
		for _, k := range kinds {
			args = append(args, k)
		}
		args = append(args, limit, offset)
		q := fmt.Sprintf(
			`SELECT id, ts, kind, data FROM memory_events WHERE kind IN (%s) ORDER BY ts DESC LIMIT ? OFFSET ?`,
			placeholders)
		rows, err = s.db.QueryContext(ctx, q, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: list memory_events: %w", err)
	}
	defer rows.Close()
	out := make([]*storage.MemoryEvent, 0, limit)
	for rows.Next() {
		var (
			id   int64
			ts   float64
			kind string
			data string
		)
		if err := rows.Scan(&id, &ts, &kind, &data); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory_event: %w", err)
		}
		out = append(out, &storage.MemoryEvent{
			ID: id, TS: fromEpoch(ts), Kind: kind, Data: []byte(data),
		})
	}
	return out, rows.Err()
}

// MemoryStats implements storage.Storage.
func (s *Store) MemoryStats(ctx context.Context) (*storage.MemoryStats, error) {
	out := &storage.MemoryStats{
		ByType:   map[string]int{},
		ByStatus: map[string]int{},
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`).Scan(&out.Total); err != nil {
		return nil, fmt.Errorf("sqlite: memory stats total: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT mem_type, COUNT(*) FROM memories GROUP BY mem_type`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: memory stats by_type: %w", err)
	}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			rows.Close()
			return nil, err
		}
		out.ByType[k] = n
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT COALESCE(status, ''), COUNT(*) FROM memories GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: memory stats by_status: %w", err)
	}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			rows.Close()
			return nil, err
		}
		if k == "" {
			k = storage.MemoryStatusActive
		}
		out.ByStatus[k] += n
	}
	rows.Close()

	var vecCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE vector IS NOT NULL AND LENGTH(vector) > 0`).Scan(&vecCount); err != nil {
		return nil, fmt.Errorf("sqlite: memory stats vector: %w", err)
	}
	if out.Total > 0 {
		out.VectorCoverage = float64(vecCount) / float64(out.Total)
	}

	hrows, err := s.db.QueryContext(ctx, `SELECT reinforcement_count, neglect_count FROM memories`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: memory stats reinforcement: %w", err)
	}
	defer hrows.Close()
	for hrows.Next() {
		var r, n int
		if err := hrows.Scan(&r, &n); err != nil {
			return nil, err
		}
		switch {
		case r == 0 && n == 0:
			out.Reinforcement.NeverUsed++
		case r > 0 && r <= 3:
			out.Reinforcement.UsedOneToFew++
		case r >= 4:
			out.Reinforcement.UsedMany++
		case r == 0 && n > 0:
			out.Reinforcement.NeglectedOnly++
		}
	}
	return out, nil
}

// MemoryHealth implements storage.Storage.
func (s *Store) MemoryHealth(ctx context.Context) (*storage.MemoryHealth, error) {
	v, err := s.schemaVersion()
	if err != nil {
		return nil, err
	}
	h := &storage.MemoryHealth{
		SchemaVersion:     v,
		MigrationsPending: v < currentSchemaVersion,
		FTSIntegrity:      "ok",
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO memories_fts(memories_fts) VALUES('integrity-check')`); err != nil {
		h.FTSIntegrity = "broken: " + err.Error()
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memories m
		 WHERE m.status = 'superseded'
		   AND m.superseded_by != ''
		   AND NOT EXISTS (SELECT 1 FROM memories WHERE id = m.superseded_by)`).Scan(&h.OrphanMemories); err != nil {
		return nil, fmt.Errorf("sqlite: memory health orphan: %w", err)
	}
	var ts float64
	var data string
	err = s.db.QueryRowContext(ctx,
		`SELECT ts, data FROM memory_events WHERE kind='memory.consolidated' ORDER BY ts DESC LIMIT 1`).Scan(&ts, &data)
	if err == nil {
		unixTS := int64(ts)
		h.ConsolidatorLastRunUnix = &unixTS
		var report storage.ConsolidateReportView
		if jerr := json.Unmarshal([]byte(data), &report); jerr == nil {
			h.ConsolidatorLastReport = &report
		}
	}
	return h, nil
}

// SkillsStats implements storage.Storage.
func (s *Store) SkillsStats(_ context.Context, skillsDir string) (*storage.SkillsStats, error) {
	out := &storage.SkillsStats{
		ByCategory: map[string]int{},
		Recent:     []storage.SkillSummary{},
	}
	if skillsDir == "" {
		return out, nil
	}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("storage: skills stats read dir: %w", err)
	}
	type row struct {
		name    string
		modUnix int64
	}
	var rows []row
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rows = append(rows, row{e.Name(), info.ModTime().Unix()})
	}
	out.Total = len(rows)
	sort.Slice(rows, func(i, j int) bool { return rows[i].modUnix > rows[j].modUnix })
	for i, r := range rows {
		if i >= 5 {
			break
		}
		out.Recent = append(out.Recent, storage.SkillSummary{Name: r.name, CreatedAt: r.modUnix})
	}
	return out, nil
}
