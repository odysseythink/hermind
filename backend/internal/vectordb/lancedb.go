//go:build !windows && !nolancedb
// +build !windows,!nolancedb

package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

type LanceDB struct {
	uri  string
	conn contracts.IConnection
}

func NewLanceDB(storageDir string) *LanceDB {
	return &LanceDB{uri: filepath.Join(storageDir, "lancedb")}
}

func (l *LanceDB) Name() string { return "lancedb" }

func (l *LanceDB) Connect(ctx context.Context) error {
	if err := os.MkdirAll(l.uri, 0755); err != nil {
		return fmt.Errorf("create lancedb dir: %w", err)
	}
	conn, err := lancedb.Connect(ctx, l.uri, nil)
	if err != nil {
		return fmt.Errorf("lancedb connect: %w", err)
	}
	l.conn = conn
	return nil
}

func (l *LanceDB) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "lancedb", "uri": l.uri}, nil
}

func (l *LanceDB) Tables(ctx context.Context) ([]string, error) {
	return l.conn.TableNames(ctx)
}

func (l *LanceDB) CountVectors(ctx context.Context, namespace string) (int64, error) {
	table, err := l.conn.OpenTable(ctx, namespace)
	if err != nil {
		return 0, nil
	}
	return table.Count(ctx)
}

func (l *LanceDB) TotalVectors(ctx context.Context) (int64, error) {
	names, err := l.conn.TableNames(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, name := range names {
		table, err := l.conn.OpenTable(ctx, name)
		if err != nil {
			continue
		}
		count, err := table.Count(ctx)
		if err != nil {
			continue
		}
		total += count
	}
	return total, nil
}

func (l *LanceDB) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	dims := len(chunks[0].Vector)

	table, err := l.openOrCreateTable(ctx, namespace, dims)
	if err != nil {
		return err
	}

	record, err := l.chunksToRecord(chunks, dims)
	if err != nil {
		return fmt.Errorf("build arrow record: %w", err)
	}
	defer record.Release()

	return table.Add(ctx, record, nil)
}

func (l *LanceDB) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}
	table, err := l.conn.OpenTable(ctx, namespace)
	if err != nil {
		return nil // namespace doesn't exist, nothing to delete
	}
	filter := fmt.Sprintf("id IN (%s)", quoteIds(vectorIds))
	return table.Delete(ctx, filter)
}

func (l *LanceDB) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}
	table, err := l.conn.OpenTable(ctx, namespace)
	if err != nil {
		return nil, err
	}

	results, err := table.VectorSearch(ctx, "vector", queryVector, opts.TopN)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	return l.parseSearchResults(results, opts)
}

func (l *LanceDB) DeleteNamespace(ctx context.Context, namespace string) error {
	return l.conn.DropTable(ctx, namespace)
}

// openOrCreateTable opens existing table or creates new one with schema.
func (l *LanceDB) openOrCreateTable(ctx context.Context, namespace string, dims int) (contracts.ITable, error) {
	table, err := l.conn.OpenTable(ctx, namespace)
	if err == nil {
		return table, nil
	}

	schema, err := lancedb.NewSchemaBuilder().
		AddStringField("id", false).
		AddVectorField("vector", dims, contracts.VectorDataTypeFloat32, false).
		AddStringField("doc_id", false).
		AddStringField("text", true).
		AddStringField("metadata", true).
		Build()
	if err != nil {
		return nil, fmt.Errorf("build schema: %w", err)
	}

	table, err = l.conn.CreateTable(ctx, namespace, schema)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	return table, nil
}

// chunksToRecord builds an Arrow Record from VectorChunks.
func (l *LanceDB) chunksToRecord(chunks []VectorChunk, dims int) (arrow.Record, error) {
	pool := memory.NewGoAllocator()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.BinaryTypes.String},
		{Name: "vector", Type: arrow.FixedSizeListOf(int32(dims), arrow.PrimitiveTypes.Float32)},
		{Name: "doc_id", Type: arrow.BinaryTypes.String},
		{Name: "text", Type: arrow.BinaryTypes.String},
		{Name: "metadata", Type: arrow.BinaryTypes.String},
	}, nil)

	b := array.NewRecordBuilder(pool, schema)
	defer b.Release()

	for _, ch := range chunks {
		b.Field(0).(*array.StringBuilder).Append(ch.ID)

		vecBuilder := b.Field(1).(*array.FixedSizeListBuilder)
		vecBuilder.Append(true)
		valueBuilder := vecBuilder.ValueBuilder().(*array.Float32Builder)
		for _, v := range ch.Vector {
			valueBuilder.Append(v)
		}

		docId, _ := ch.Metadata["docId"].(string)
		b.Field(2).(*array.StringBuilder).Append(docId)

		text, _ := ch.Metadata["text"].(string)
		b.Field(3).(*array.StringBuilder).Append(text)

		// metadata as JSON string
		metadataStr := ""
		if ch.Metadata != nil {
			metaJSON, err := json.Marshal(ch.Metadata)
			if err != nil {
				return nil, fmt.Errorf("marshal metadata for chunk %s: %w", ch.ID, err)
			}
			metadataStr = string(metaJSON)
		}
		b.Field(4).(*array.StringBuilder).Append(metadataStr)
	}

	return b.NewRecord(), nil
}

// parseSearchResults extracts SearchResult from SDK query results.
func (l *LanceDB) parseSearchResults(rows []map[string]interface{}, opts SearchOptions) ([]SearchResult, error) {
	var results []SearchResult

	for _, row := range rows {
		score := 0.0
		if dist, ok := row["_distance"].(float64); ok {
			score = distanceToSimilarity(dist)
		} else if dist32, ok := row["_distance"].(float32); ok {
			score = distanceToSimilarity(float64(dist32))
		}

		if score < opts.SimilarityThreshold {
			continue
		}

		meta := map[string]any{}
		if metaStr, ok := row["metadata"].(string); ok && metaStr != "" {
			json.Unmarshal([]byte(metaStr), &meta)
		}

		docId := ""
		if d, ok := row["doc_id"].(string); ok {
			docId = d
		}
		text := ""
		if t, ok := row["text"].(string); ok {
			text = t
		}

		results = append(results, SearchResult{
			DocId:    docId,
			Text:     text,
			Score:    score,
			Distance: 1.0 - score,
			Metadata: meta,
		})
	}

	return results, nil
}

func quoteIds(ids []string) string {
	var result string
	for i, id := range ids {
		if i > 0 {
			result += ","
		}
		// Escape single quotes by doubling them
		safeId := strings.ReplaceAll(id, "'", "''")
		result += fmt.Sprintf("'%s'", safeId)
	}
	return result
}
