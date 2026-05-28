package reranker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/pantheon/providers/openaicompat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopReranker_PassthroughTopN(t *testing.T) {
	ctx := context.Background()
	r := &NoopReranker{}
	docs := []string{"a", "b", "c", "d"}
	result, err := r.Rerank(ctx, "q", docs, 2)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 0, result[0].Index)
	assert.Equal(t, "a", result[0].Text)
	assert.Equal(t, 1, result[1].Index)
	assert.Equal(t, "b", result[1].Text)
}

func TestNoopReranker_TopNExceedsLen_ReturnsAll(t *testing.T) {
	ctx := context.Background()
	r := &NoopReranker{}
	docs := []string{"a", "b"}
	result, err := r.Rerank(ctx, "q", docs, 10)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 0, result[0].Index)
	assert.Equal(t, 1, result[1].Index)
}

func TestPantheonReranker_NilModel_ReturnsError(t *testing.T) {
	ctx := context.Background()
	r := NewPantheonReranker(nil)
	_, err := r.Rerank(ctx, "q", []string{"doc"}, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil rerank model")
}

func TestPantheonReranker_EmptyDocs_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	r := NewPantheonReranker(&openAICompatRerankModel{})
	result, err := r.Rerank(ctx, "q", []string{}, 3)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestPantheonReranker_Rerank_ReordersDocs(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "test",
			"results": []map[string]any{
				{"index": 2, "relevance_score": 0.95},
				{"index": 0, "relevance_score": 0.7},
				{"index": 1, "relevance_score": 0.3},
			},
		})
	}))
	defer srv.Close()

	client := openaicompat.NewClient(srv.URL, "test-key")
	client.RerankPath = "/v2/rerank"
	client.RerankFormat = openaicompat.RerankFormatCohereV2
	model := &openAICompatRerankModel{client: client, model: "rerank-english-v3.0"}
	reranker := NewPantheonReranker(model)
	result, err := reranker.Rerank(ctx, "query", []string{"doc0", "doc1", "doc2"}, 3)
	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, 2, result[0].Index)
	assert.InDelta(t, 0.95, result[0].Score, 0.001)
	assert.Equal(t, "doc2", result[0].Text)
	assert.Equal(t, 0, result[1].Index)
	assert.InDelta(t, 0.7, result[1].Score, 0.001)
	assert.Equal(t, "doc0", result[1].Text)
	assert.Equal(t, 1, result[2].Index)
	assert.InDelta(t, 0.3, result[2].Score, 0.001)
	assert.Equal(t, "doc1", result[2].Text)
}
