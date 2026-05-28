package embedder

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/pantheon/extensions/embed"
)

// Embedder generates vector embeddings for text.
type Embedder interface {
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
	Dimensions() int
}

// PantheonEmbedder wraps a Pantheon embed.EmbeddingModel.
type PantheonEmbedder struct {
	model      embed.EmbeddingModel
	dimensions int
}

// Deprecated: use NewEmbedder which supports multiple providers.
func NewPantheonEmbedder(cfg *config.Config) (*PantheonEmbedder, error) {
	e, err := NewEmbedder(cfg, nil)
	if err != nil {
		return nil, err
	}
	return e.(*PantheonEmbedder), nil
}

// EmbedTexts embeds multiple texts in a single batch.
func (e *PantheonEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	resp, err := e.model.Embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed texts: %w", err)
	}
	if e.dimensions == 0 && len(resp.Embeddings) > 0 {
		e.dimensions = len(resp.Embeddings[0])
	}
	return resp.Embeddings, nil
}

// EmbedQuery embeds a single query text.
func (e *PantheonEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	embeddings, err := e.EmbedTexts(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	return embeddings[0], nil
}

// Dimensions returns the embedding vector size.
func (e *PantheonEmbedder) Dimensions() int {
	return e.dimensions
}
