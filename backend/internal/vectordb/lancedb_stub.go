//go:build windows || nolancedb
// +build windows nolancedb

package vectordb

import (
	"context"
	"fmt"
	"path/filepath"
)

type LanceDB struct {
	uri string
}

func NewLanceDB(storageDir string) *LanceDB {
	return &LanceDB{uri: filepath.Join(storageDir, "lancedb")}
}

func (l *LanceDB) Name() string                      { return "lancedb" }
func (l *LanceDB) Connect(ctx context.Context) error { return fmt.Errorf("lancedb: stub build") }
func (l *LanceDB) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "lancedb", "uri": l.uri}, nil
}
func (l *LanceDB) Tables(ctx context.Context) ([]string, error)                      { return nil, nil }
func (l *LanceDB) CountVectors(ctx context.Context, namespace string) (int64, error) { return 0, nil }
func (l *LanceDB) TotalVectors(ctx context.Context) (int64, error)                   { return 0, nil }
func (l *LanceDB) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	return fmt.Errorf("lancedb: stub build")
}
func (l *LanceDB) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	return nil
}
func (l *LanceDB) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	return nil, fmt.Errorf("lancedb: stub build")
}
func (l *LanceDB) DeleteNamespace(ctx context.Context, namespace string) error { return nil }
