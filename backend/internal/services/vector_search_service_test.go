package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockVectorDBForSearch struct {
	results []vectordb.SearchResult
}

func (m *mockVectorDBForSearch) Name() string                                              { return "mock" }
func (m *mockVectorDBForSearch) Connect(ctx context.Context) error                         { return nil }
func (m *mockVectorDBForSearch) Heartbeat(ctx context.Context) (map[string]any, error)     { return nil, nil }
func (m *mockVectorDBForSearch) AddVectors(ctx context.Context, namespace string, chunks []vectordb.VectorChunk) error {
	return nil
}
func (m *mockVectorDBForSearch) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	return nil
}
func (m *mockVectorDBForSearch) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts vectordb.SearchOptions) ([]vectordb.SearchResult, error) {
	return m.results, nil
}
func (m *mockVectorDBForSearch) DeleteNamespace(ctx context.Context, namespace string) error { return nil }
func (m *mockVectorDBForSearch) Tables(ctx context.Context) ([]string, error)                { return nil, nil }
func (m *mockVectorDBForSearch) CountVectors(ctx context.Context, namespace string) (int64, error) {
	return int64(len(m.results)), nil
}
func (m *mockVectorDBForSearch) TotalVectors(ctx context.Context) (int64, error) { return 0, nil }

type mockEmbedderForSearch struct{}

func (m *mockEmbedderForSearch) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}
func (m *mockEmbedderForSearch) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
func (m *mockEmbedderForSearch) Dimensions() int { return 3 }

func TestVectorSearchService_WithNoopReranker_ReturnsOriginalOrder(t *testing.T) {
	cfg := &config.Config{}
	vec := NewVectorService(cfg)
	mockDB := &mockVectorDBForSearch{
		results: []vectordb.SearchResult{
			{DocId: "d1", Text: "first", Score: 0.9},
			{DocId: "d2", Text: "second", Score: 0.8},
			{DocId: "d3", Text: "third", Score: 0.7},
		},
	}
	vec.SetProvider(mockDB)

	svc := NewVectorSearchService(vec, &mockEmbedderForSearch{}, &reranker.NoopReranker{})

	ws := &models.Workspace{Name: "ws", Slug: "ws"}
	results, err := svc.Search(context.Background(), ws, dto.VectorSearchRequest{Query: "test"})
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "d1", results[0].ID)
	assert.Equal(t, "d2", results[1].ID)
	assert.Equal(t, "d3", results[2].ID)
}
