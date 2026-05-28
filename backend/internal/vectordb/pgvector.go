package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgvectorTableName = "anythingllm_vectors"

type PGVector struct {
	pool    *pgxpool.Pool
	connStr string
}

func NewPGVector(connStr string) *PGVector {
	return &PGVector{connStr: connStr}
}

func (p *PGVector) Name() string { return "pgvector" }

func (p *PGVector) Connect(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, p.connStr)
	if err != nil {
		return fmt.Errorf("pgx connect: %w", err)
	}
	p.pool = pool
	_, err = p.pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("create extension: %w", err)
	}
	return nil
}

func (p *PGVector) Heartbeat(ctx context.Context) (map[string]any, error) {
	var version string
	if err := p.pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return nil, err
	}
	return map[string]any{"name": "pgvector", "version": version}, nil
}

func (p *PGVector) Tables(ctx context.Context) ([]string, error) {
	rows, err := p.pool.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (p *PGVector) CountVectors(ctx context.Context, namespace string) (int64, error) {
	var count int64
	err := p.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(id) FROM %s WHERE namespace = $1", pgvectorTableName), namespace).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (p *PGVector) TotalVectors(ctx context.Context) (int64, error) {
	var count int64
	err := p.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(id) FROM %s", pgvectorTableName)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (p *PGVector) createTableIfNotExists(ctx context.Context, dimensions int) error {
	_, err := p.pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY,
			namespace TEXT NOT NULL,
			embedding vector(%d),
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, pgvectorTableName, dimensions))
	if err != nil {
		return err
	}
	// Create HNSW index for approximate nearest neighbor search.
	_, err = p.pool.Exec(ctx, fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_embedding ON %s
		USING hnsw (embedding vector_cosine_ops)
	`, pgvectorTableName, pgvectorTableName))
	return err
}

func (p *PGVector) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	dims := len(chunks[0].Vector)
	if err := p.createTableIfNotExists(ctx, dims); err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	batch := &pgx.Batch{}
	for _, ch := range chunks {
		metaJSON, _ := json.Marshal(sanitizeForJSONB(ch.Metadata))
		batch.Queue(
			fmt.Sprintf("INSERT INTO %s (id, namespace, embedding, metadata) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING", pgvectorTableName),
			ch.ID, namespace, vecToString(ch.Vector), metaJSON,
		)
	}
	br := p.pool.SendBatch(ctx, batch)
	return br.Close()
}

func (p *PGVector) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}
	placeholders := make([]string, len(vectorIds))
	args := make([]interface{}, len(vectorIds))
	for i, id := range vectorIds {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE namespace = $%d AND id IN (%s)",
		pgvectorTableName, len(vectorIds)+1, strings.Join(placeholders, ","),
	)
	args = append(args, namespace)
	_, err := p.pool.Exec(ctx, query, args...)
	return err
}

func (p *PGVector) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}
	limit := opts.TopN

	query := fmt.Sprintf(
		"SELECT embedding <=> $1 AS _distance, metadata FROM %s WHERE namespace = $2 ORDER BY _distance ASC LIMIT $3",
		pgvectorTableName,
	)
	rows, err := p.pool.Query(ctx, query, vecToString(queryVector), namespace, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var distance float64
		var metaBytes []byte
		if err := rows.Scan(&distance, &metaBytes); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		var meta map[string]interface{}
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}

		score := 1.0 - distance // cosine distance to similarity
		if score < opts.SimilarityThreshold {
			continue
		}

		text, _ := meta["text"].(string)
		docId, _ := meta["docId"].(string)
		results = append(results, SearchResult{
			DocId:    docId,
			Text:     text,
			Score:    score,
			Distance: distance,
			Metadata: meta,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	return results, nil
}

func (p *PGVector) DeleteNamespace(ctx context.Context, namespace string) error {
	_, err := p.pool.Exec(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE namespace = $1", pgvectorTableName),
		namespace,
	)
	return err
}

// vecToString converts []float32 to pgvector string format [1.0,2.0,...]
func vecToString(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%g", f))
	}
	b.WriteByte(']')
	return b.String()
}

// sanitizeForJSONB strips control characters that PostgreSQL JSONB rejects.
func sanitizeForJSONB(v interface{}) interface{} {
	switch x := v.(type) {
	case string:
		var sb strings.Builder
		for _, r := range x {
			if r == 0x00 || (r < 0x20 && r != '\t' && r != '\n' && r != '\r') {
				continue
			}
			sb.WriteRune(r)
		}
		return sb.String()
	case map[string]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, v := range x {
			m[k] = sanitizeForJSONB(v)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(x))
		for i, v := range x {
			a[i] = sanitizeForJSONB(v)
		}
		return a
	default:
		return v
	}
}
