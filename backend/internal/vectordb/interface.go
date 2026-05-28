package vectordb

import "context"

type VectorChunk struct {
	ID       string
	Vector   []float32
	Metadata map[string]any
}

type SearchResult struct {
	DocId    string
	Text     string
	Score    float64
	Distance float64
	Metadata map[string]any
}

type SearchOptions struct {
	SimilarityThreshold float64
	TopN                int
	FilterIdentifiers   []string
}

type VectorDatabase interface {
	Name() string
	Connect(ctx context.Context) error
	Heartbeat(ctx context.Context) (map[string]any, error)
	AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error
	DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error
	SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error)
	DeleteNamespace(ctx context.Context, namespace string) error
	Tables(ctx context.Context) ([]string, error)
	CountVectors(ctx context.Context, namespace string) (int64, error)
	TotalVectors(ctx context.Context) (int64, error)
}
