// storage/sqlite/memory.go
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/embedding"
)

// SaveMemory inserts or updates a memory entry. If the ID is empty, a new
// ID is generated using the epoch nanoseconds as a string.
func (s *Store) SaveMemory(ctx context.Context, m *storage.Memory) error {
	if m.ID == "" {
		return fmt.Errorf("sqlite: memory ID is required")
	}
	tagsJSON, _ := json.Marshal(m.Tags)
	metaStr := string(m.Metadata)
	if metaStr == "" {
		metaStr = "{}"
	}
	status := m.Status
	if status == "" {
		status = storage.MemoryStatusActive
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO memories (id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector, status, superseded_by, reinforcement_count, neglect_count, last_used_at, reinforced_at_seq)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            content = excluded.content,
            category = excluded.category,
            tags = excluded.tags,
            metadata = excluded.metadata,
            updated_at = excluded.updated_at,
            mem_type = excluded.mem_type,
            vector = excluded.vector,
            status = excluded.status,
            superseded_by = excluded.superseded_by,
            reinforcement_count = excluded.reinforcement_count,
            neglect_count = excluded.neglect_count,
            last_used_at = excluded.last_used_at,
            reinforced_at_seq = excluded.reinforced_at_seq
    `,
		m.ID, m.UserID, m.Content, m.Category, string(tagsJSON), metaStr,
		toEpoch(m.CreatedAt), toEpoch(m.UpdatedAt), m.MemType, m.Vector,
		status, m.SupersededBy, m.ReinforcementCount, m.NeglectCount, toEpoch(m.LastUsedAt), m.ReinforcedAtSeq,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save memory %s: %w", m.ID, err)
	}
	return nil
}

// MarkMemorySuperseded sets oldID → superseded by newID in a single update.
// Returns ErrNotFound if oldID does not exist.
func (s *Store) MarkMemorySuperseded(ctx context.Context, oldID, newID string) error {
	res, err := s.db.ExecContext(ctx, `
        UPDATE memories SET status = ?, superseded_by = ?, updated_at = ?
        WHERE id = ?`,
		storage.MemoryStatusSuperseded, newID, toEpoch(time.Now().UTC()), oldID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: mark superseded %s: %w", oldID, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// BumpMemoryUsage implements storage.Storage.
func (s *Store) BumpMemoryUsage(ctx context.Context, id string, used bool) error {
	var (
		res sql.Result
		err error
	)
	if used {
		res, err = s.db.ExecContext(ctx,
			`UPDATE memories
			    SET reinforcement_count = reinforcement_count + 1,
			        last_used_at        = ?,
			        reinforced_at_seq   = COALESCE(
			            (SELECT seq FROM skills_generation WHERE id = 1), 0)
			  WHERE id = ?`,
			toEpoch(time.Now().UTC()), id)
	} else {
		res, err = s.db.ExecContext(ctx,
			`UPDATE memories SET neglect_count = neglect_count + 1 WHERE id = ?`, id)
	}
	if err != nil {
		return fmt.Errorf("sqlite: bump memory %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// GetMemory fetches a memory by ID.
func (s *Store) GetMemory(ctx context.Context, id string) (*storage.Memory, error) {
	var (
		m        storage.Memory
		tagsJSON string
		metaStr  string
		created  float64
		updated  float64
		vecBytes []byte
		lastUsed float64
	)
	err := s.db.QueryRowContext(ctx, `
        SELECT id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector, status, superseded_by, reinforcement_count, neglect_count, last_used_at, reinforced_at_seq
        FROM memories WHERE id = ?`, id,
	).Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &tagsJSON, &metaStr, &created, &updated, &m.MemType, &vecBytes, &m.Status, &m.SupersededBy, &m.ReinforcementCount, &m.NeglectCount, &lastUsed, &m.ReinforcedAtSeq)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get memory %s: %w", id, err)
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	m.Metadata = []byte(metaStr)
	m.CreatedAt = fromEpoch(created)
	m.UpdatedAt = fromEpoch(updated)
	m.Vector = vecBytes
	m.LastUsedAt = fromEpoch(lastUsed)
	return &m, nil
}

// scoredMemory pairs a memory with per-signal raw scores used to build
// the hybrid ranking in SearchMemories.
type scoredMemory struct {
	mem           *storage.Memory
	fts           float64 // -bm25 (higher = more relevant); 0 when query is empty
	cosine        float64 // 0 when no vector
	recency       float64 // seconds-since-epoch, used only for relative ordering
	reinforcement float64 // (reinforcement - neglect) / max(sum, 1)
}

// SearchMemories runs a hybrid FTS5 + cosine + recency + reinforcement search.
//
// When opts.QueryVector is set we fetch limit*3 candidates via FTS (or
// a recency scan when the query is empty), score each candidate on four
// independently min-max-normalized signals, and return the top limit by
// weighted sum: 0.35 FTS + 0.40 cosine + 0.10 recency + 0.15 reinforcement.
//
// When QueryVector is nil we fall back to the legacy FTS-ordered path.
func (s *Store) SearchMemories(ctx context.Context, query string, opts *storage.MemorySearchOptions) ([]*storage.Memory, error) {
	limit := 20
	fetchLimit := limit
	var queryVec []float32
	if opts != nil {
		if opts.Limit > 0 {
			limit = opts.Limit
		}
		queryVec = opts.QueryVector
		if queryVec != nil {
			fetchLimit = limit * 3
		}
	}

	candidates, err := s.fetchMemoryCandidates(ctx, query, opts, fetchLimit)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Legacy path: no query vector → pure FTS ordering already applied.
	if queryVec == nil {
		out := make([]*storage.Memory, 0, len(candidates))
		for _, c := range candidates {
			out = append(out, c.mem)
		}
		return out, nil
	}

	// Hybrid scoring: fold cosine into each candidate, then min-max-normalize.
	for _, c := range candidates {
		if len(c.mem.Vector) == 0 {
			continue
		}
		v, err := embedding.DecodeVector(c.mem.Vector)
		if err != nil || len(v) == 0 {
			continue
		}
		c.cosine = float64(embedding.CosineSimilarity(v, queryVec))
	}

	minF, maxF := scoreRange(candidates, func(c *scoredMemory) float64 { return c.fts })
	minC, maxC := scoreRange(candidates, func(c *scoredMemory) float64 { return c.cosine })
	minR, maxR := scoreRange(candidates, func(c *scoredMemory) float64 { return c.recency })
	minRe, maxRe := scoreRange(candidates, func(c *scoredMemory) float64 { return c.reinforcement })

	sort.SliceStable(candidates, func(i, j int) bool {
		a := 0.35*normalize(candidates[i].fts, minF, maxF) +
			0.40*normalize(candidates[i].cosine, minC, maxC) +
			0.10*normalize(candidates[i].recency, minR, maxR) +
			0.15*normalize(candidates[i].reinforcement, minRe, maxRe)
		b := 0.35*normalize(candidates[j].fts, minF, maxF) +
			0.40*normalize(candidates[j].cosine, minC, maxC) +
			0.10*normalize(candidates[j].recency, minR, maxR) +
			0.15*normalize(candidates[j].reinforcement, minRe, maxRe)
		return a > b
	})

	out := make([]*storage.Memory, 0, limit)
	for i, c := range candidates {
		if i >= limit {
			break
		}
		out = append(out, c.mem)
	}
	return out, nil
}

// fetchMemoryCandidates runs the FTS/recency query and scans rows into
// scoredMemory entries with raw fts and recency signals populated.
func (s *Store) fetchMemoryCandidates(ctx context.Context, query string, opts *storage.MemorySearchOptions, fetchLimit int) ([]*scoredMemory, error) {
	includeAll := opts != nil && opts.IncludeAll
	var rows *sql.Rows
	var err error
	if query == "" {
		var wheres []string
		var args []any
		if opts != nil && opts.UserID != "" {
			wheres = append(wheres, "user_id = ?")
			args = append(args, opts.UserID)
		}
		if !includeAll {
			wheres = append(wheres, "status = ?")
			args = append(args, storage.MemoryStatusActive)
		}
		whereSQL := ""
		if len(wheres) > 0 {
			whereSQL = " WHERE " + strings.Join(wheres, " AND ")
		}
		args = append(args, fetchLimit)
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector, status, superseded_by, reinforcement_count, neglect_count, last_used_at, reinforced_at_seq, 0 as rank
             FROM memories`+whereSQL+` ORDER BY created_at DESC LIMIT ?`, args...)
	} else {
		wheres := []string{"memories_fts MATCH ?"}
		args := []any{query}
		if opts != nil && opts.UserID != "" {
			wheres = append(wheres, "m.user_id = ?")
			args = append(args, opts.UserID)
		}
		if !includeAll {
			wheres = append(wheres, "m.status = ?")
			args = append(args, storage.MemoryStatusActive)
		}
		args = append(args, fetchLimit)
		rows, err = s.db.QueryContext(ctx,
			`SELECT m.id, m.user_id, m.content, m.category, m.tags, m.metadata, m.created_at, m.updated_at, m.mem_type, m.vector, m.status, m.superseded_by, m.reinforcement_count, m.neglect_count, m.last_used_at, m.reinforced_at_seq, bm25(memories_fts) as rank
             FROM memories_fts
             JOIN memories m ON m.rowid = memories_fts.rowid
             WHERE `+strings.Join(wheres, " AND ")+` ORDER BY rank LIMIT ?`, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: search memories: %w", err)
	}
	defer rows.Close()

	var out []*scoredMemory
	for rows.Next() {
		var (
			m        storage.Memory
			tagsJSON string
			metaStr  string
			created  float64
			updated  float64
			vecBytes []byte
			lastUsed float64
			rank     float64
		)
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &tagsJSON, &metaStr, &created, &updated, &m.MemType, &vecBytes, &m.Status, &m.SupersededBy, &m.ReinforcementCount, &m.NeglectCount, &lastUsed, &m.ReinforcedAtSeq, &rank); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		m.Metadata = []byte(metaStr)
		m.CreatedAt = fromEpoch(created)
		m.UpdatedAt = fromEpoch(updated)
		m.Vector = vecBytes
		m.LastUsedAt = fromEpoch(lastUsed)

		if opts != nil && len(opts.Tags) > 0 && !hasAnyTag(m.Tags, opts.Tags) {
			continue
		}
		rein := float64(m.ReinforcementCount - m.NeglectCount)
		denom := float64(m.ReinforcementCount + m.NeglectCount)
		if denom < 1 {
			denom = 1
		}
		out = append(out, &scoredMemory{
			mem:           &m,
			fts:           -rank,
			recency:       float64(m.CreatedAt.Unix()),
			reinforcement: rein / denom,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scoreRange(cs []*scoredMemory, get func(*scoredMemory) float64) (float64, float64) {
	if len(cs) == 0 {
		return 0, 0
	}
	min, max := math.Inf(1), math.Inf(-1)
	for _, c := range cs {
		v := get(c)
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

func normalize(v, min, max float64) float64 {
	if max <= min {
		return 0
	}
	return (v - min) / (max - min)
}

// DeleteMemory removes a memory by ID.
func (s *Store) DeleteMemory(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete memory %s: %w", id, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// ListMemoriesByType returns active memories filtered by MemType, newest first.
func (s *Store) ListMemoriesByType(ctx context.Context, memType string, limit int) ([]*storage.Memory, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector, status, superseded_by, reinforcement_count, neglect_count, last_used_at, reinforced_at_seq
        FROM memories WHERE mem_type = ? AND status = ? ORDER BY created_at DESC LIMIT ?`,
		memType, storage.MemoryStatusActive, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list memories by type: %w", err)
	}
	defer rows.Close()
	var out []*storage.Memory
	for rows.Next() {
		var (
			m        storage.Memory
			tagsJSON string
			metaStr  string
			created  float64
			updated  float64
			vecBytes []byte
			lastUsed float64
		)
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.Category,
			&tagsJSON, &metaStr, &created, &updated, &m.MemType, &vecBytes, &m.Status, &m.SupersededBy, &m.ReinforcementCount, &m.NeglectCount, &lastUsed, &m.ReinforcedAtSeq); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		m.Metadata = []byte(metaStr)
		m.CreatedAt = fromEpoch(created)
		m.UpdatedAt = fromEpoch(updated)
		m.Vector = vecBytes
		m.LastUsedAt = fromEpoch(lastUsed)
		out = append(out, &m)
	}
	return out, rows.Err()
}

// hasAnyTag returns true if any of the want tags appears in have.
func hasAnyTag(have []string, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if strings.EqualFold(h, w) {
				return true
			}
		}
	}
	return false
}
