// storage/sqlite/memory.go
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO memories (id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            content = excluded.content,
            category = excluded.category,
            tags = excluded.tags,
            metadata = excluded.metadata,
            updated_at = excluded.updated_at,
            mem_type = excluded.mem_type,
            vector = excluded.vector
    `,
		m.ID, m.UserID, m.Content, m.Category, string(tagsJSON), metaStr,
		toEpoch(m.CreatedAt), toEpoch(m.UpdatedAt), m.MemType, m.Vector,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save memory %s: %w", m.ID, err)
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
	)
	err := s.db.QueryRowContext(ctx, `
        SELECT id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector
        FROM memories WHERE id = ?`, id,
	).Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &tagsJSON, &metaStr, &created, &updated, &m.MemType, &vecBytes)
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
	return &m, nil
}

// SearchMemories runs an FTS5 match against the memories table.
// If opts.QueryVector is set, fetches limit*3 candidates and reranks by cosine similarity.
// Returns matches ordered by created_at DESC (or by similarity if QueryVector is set).
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
		} else {
			fetchLimit = limit
		}
	}

	// If query is empty, list recent memories instead of running FTS.
	var rows *sql.Rows
	var err error
	if query == "" {
		where := ""
		args := []any{}
		if opts != nil && opts.UserID != "" {
			where = " WHERE user_id = ?"
			args = append(args, opts.UserID)
		}
		args = append(args, fetchLimit)
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector
             FROM memories`+where+` ORDER BY created_at DESC LIMIT ?`, args...)
	} else {
		where := ""
		args := []any{query}
		if opts != nil && opts.UserID != "" {
			where = " AND m.user_id = ?"
			args = append(args, opts.UserID)
		}
		args = append(args, fetchLimit)
		rows, err = s.db.QueryContext(ctx,
			`SELECT m.id, m.user_id, m.content, m.category, m.tags, m.metadata, m.created_at, m.updated_at, m.mem_type, m.vector
             FROM memories_fts
             JOIN memories m ON m.rowid = memories_fts.rowid
             WHERE memories_fts MATCH ?`+where+` ORDER BY m.created_at DESC LIMIT ?`, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: search memories: %w", err)
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
		)
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.Category, &tagsJSON, &metaStr, &created, &updated, &m.MemType, &vecBytes); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		m.Metadata = []byte(metaStr)
		m.CreatedAt = fromEpoch(created)
		m.UpdatedAt = fromEpoch(updated)
		m.Vector = vecBytes

		// Optional tag filter (post-query, since tags are stored as JSON)
		if opts != nil && len(opts.Tags) > 0 && !hasAnyTag(m.Tags, opts.Tags) {
			continue
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Vector reranking: if QueryVector is set, rerank and trim to limit
	if queryVec != nil && len(out) > 0 {
		ranked := embedding.Rerank(out, func(m *storage.Memory) []float32 {
			if len(m.Vector) == 0 {
				return nil
			}
			v, _ := embedding.DecodeVector(m.Vector)
			return v
		}, queryVec)
		out = make([]*storage.Memory, 0, limit)
		for i, r := range ranked {
			if i >= limit {
				break
			}
			out = append(out, r.Value)
		}
	}
	return out, nil
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

// ListMemoriesByType returns memories filtered by MemType, newest first.
func (s *Store) ListMemoriesByType(ctx context.Context, memType string, limit int) ([]*storage.Memory, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, user_id, content, category, tags, metadata, created_at, updated_at, mem_type, vector
        FROM memories WHERE mem_type = ? ORDER BY created_at DESC LIMIT ?`, memType, limit)
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
		)
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.Category,
			&tagsJSON, &metaStr, &created, &updated, &m.MemType, &vecBytes); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory: %w", err)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		m.Metadata = []byte(metaStr)
		m.CreatedAt = fromEpoch(created)
		m.UpdatedAt = fromEpoch(updated)
		m.Vector = vecBytes
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
