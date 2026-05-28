package reranker

import (
	"context"
	"fmt"

	"github.com/odysseythink/pantheon/extensions/rerank"
	"github.com/odysseythink/pantheon/providers/openaicompat"
)

// ScoredDocument is a document with its relevance score.
type ScoredDocument struct {
	Index int
	Score float64
	Text  string
}

// Reranker reorders a list of documents by relevance to a query.
type Reranker interface {
	Rerank(ctx context.Context, query string, documents []string, topN int) ([]ScoredDocument, error)
}

// NoopReranker is a passthrough reranker.
type NoopReranker struct{}

func (n *NoopReranker) Rerank(ctx context.Context, query string, docs []string, topN int) ([]ScoredDocument, error) {
	if topN <= 0 || topN > len(docs) {
		topN = len(docs)
	}
	out := make([]ScoredDocument, 0, topN)
	for i := 0; i < topN; i++ {
		out = append(out, ScoredDocument{Index: i, Score: 0, Text: docs[i]})
	}
	return out, nil
}

// PantheonReranker wraps a pantheon rerank model.
type PantheonReranker struct {
	model rerank.RerankModel
}

func NewPantheonReranker(model rerank.RerankModel) *PantheonReranker {
	return &PantheonReranker{model: model}
}

func (p *PantheonReranker) Rerank(ctx context.Context, query string, docs []string, topN int) ([]ScoredDocument, error) {
	if p.model == nil {
		return nil, fmt.Errorf("nil rerank model")
	}
	if len(docs) == 0 {
		return nil, nil
	}
	if topN <= 0 || topN > len(docs) {
		topN = len(docs)
	}
	resp, err := p.model.Rerank(ctx, &rerank.RerankRequest{
		Query:           query,
		Documents:       docs,
		TopN:            topN,
		ReturnDocuments: true,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ScoredDocument, 0, len(resp.Results))
	for _, r := range resp.Results {
		text := ""
		if r.Index >= 0 && r.Index < len(docs) {
			text = docs[r.Index]
		}
		out = append(out, ScoredDocument{Index: r.Index, Score: float64(r.RelevanceScore), Text: text})
	}
	return out, nil
}

// openAICompatRerankModel adapts openaicompat.Client to rerank.RerankModel.
type openAICompatRerankModel struct {
	client *openaicompat.Client
	model  string
}

func (m *openAICompatRerankModel) Rerank(ctx context.Context, req *rerank.RerankRequest) (*rerank.RerankResponse, error) {
	return m.client.CreateRerank(ctx, m.model, req)
}
