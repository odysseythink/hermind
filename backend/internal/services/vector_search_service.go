package services

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/embedder"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/mlog"
)

type VectorSearchService struct {
	vectorSvc *VectorService
	embedder  embedder.Embedder
	reranker  reranker.Reranker
}

func NewVectorSearchService(vectorSvc *VectorService, embedder embedder.Embedder, reranker reranker.Reranker) *VectorSearchService {
	return &VectorSearchService{vectorSvc: vectorSvc, embedder: embedder, reranker: reranker}
}

func (s *VectorSearchService) Search(ctx context.Context, ws *models.Workspace, req dto.VectorSearchRequest) ([]dto.VectorSearchResult, error) {
	if s.vectorSvc.provider == nil {
		return nil, fmt.Errorf("vector provider not connected")
	}

	queryVector, err := s.embedder.EmbedQuery(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query failed: %w", err)
	}

	topN := 4
	if req.TopN != nil {
		topN = *req.TopN
	}
	if ws.TopN != nil {
		topN = *ws.TopN
	}

	threshold := 0.25
	if req.ScoreThreshold != nil {
		threshold = *req.ScoreThreshold
	}
	if ws.SimilarityThreshold != nil {
		threshold = *ws.SimilarityThreshold
	}

	count, err := s.vectorSvc.CountVectors(ctx, ws.Slug)
	if err != nil || count == 0 {
		return []dto.VectorSearchResult{}, nil
	}

	results, err := s.vectorSvc.SimilaritySearch(ctx, ws.Slug, queryVector, vectordb.SearchOptions{
		TopN:                topN,
		SimilarityThreshold: threshold,
	})
	if err != nil {
		return nil, err
	}

	if s.reranker != nil {
		texts := make([]string, len(results))
		for i, r := range results {
			texts[i] = r.Text
		}
		if ranked, err := s.reranker.Rerank(ctx, req.Query, texts, topN); err == nil {
			reordered := make([]vectordb.SearchResult, 0, len(ranked))
			for _, rr := range ranked {
				if rr.Index >= 0 && rr.Index < len(results) {
					reordered = append(reordered, results[rr.Index])
				}
			}
			results = reordered
		} else {
			mlog.Warning("rerank failed, using raw search results", mlog.Err(err))
		}
	}

	out := make([]dto.VectorSearchResult, len(results))
	for i, r := range results {
		out[i] = dto.VectorSearchResult{
			ID:       r.DocId,
			Text:     r.Text,
			Metadata: r.Metadata,
			Distance: r.Distance,
			Score:    r.Score,
		}
	}
	return out, nil
}
