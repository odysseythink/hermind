package vectordb

import (
	"context"
	"fmt"

	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

type Pinecone struct {
	apiKey string
	host   string
	client *pinecone.Client
	index  *pinecone.IndexConnection
}

func NewPinecone(apiKey, host string) *Pinecone {
	return &Pinecone{apiKey: apiKey, host: host}
}

func (p *Pinecone) Name() string { return "pinecone" }

func (p *Pinecone) Connect(ctx context.Context) error {
	client, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: p.apiKey,
	})
	if err != nil {
		return fmt.Errorf("pinecone: create client: %w", err)
	}

	idxConn, err := client.Index(pinecone.NewIndexConnParams{
		Host: p.host,
	})
	if err != nil {
		return fmt.Errorf("pinecone: connect to index: %w", err)
	}

	p.client = client
	p.index = idxConn
	return nil
}

func (p *Pinecone) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "pinecone"}, nil
}

func (p *Pinecone) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	idxConn, err := p.indexConn(namespace)
	if err != nil {
		return err
	}

	vectors := make([]*pinecone.Vector, len(chunks))
	for i, chunk := range chunks {
		var metadata *structpb.Struct
		if chunk.Metadata != nil {
			metadata, err = structpb.NewStruct(chunk.Metadata)
			if err != nil {
				return fmt.Errorf("pinecone: convert metadata for vector %s: %w", chunk.ID, err)
			}
		}

		vectors[i] = &pinecone.Vector{
			Id:       chunk.ID,
			Values:   chunk.Vector,
			Metadata: metadata,
		}
	}

	_, err = idxConn.UpsertVectors(ctx, vectors)
	if err != nil {
		return fmt.Errorf("pinecone: upsert vectors: %w", err)
	}
	return nil
}

func (p *Pinecone) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}

	idxConn, err := p.indexConn(namespace)
	if err != nil {
		return err
	}

	err = idxConn.DeleteVectorsById(ctx, vectorIds)
	if err != nil {
		return fmt.Errorf("pinecone: delete vectors: %w", err)
	}
	return nil
}

func (p *Pinecone) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}

	idxConn, err := p.indexConn(namespace)
	if err != nil {
		return nil, err
	}

	res, err := idxConn.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          queryVector,
		TopK:            uint32(opts.TopN),
		IncludeMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("pinecone: query vectors: %w", err)
	}

	var results []SearchResult
	for _, match := range res.Matches {
		score := float64(match.Score)
		if score < opts.SimilarityThreshold {
			continue
		}

		meta := map[string]any{}
		if match.Vector != nil && match.Vector.Metadata != nil {
			meta = match.Vector.Metadata.AsMap()
		}

		docId := ""
		text := ""
		if d, ok := meta["docId"].(string); ok {
			docId = d
		}
		if t, ok := meta["text"].(string); ok {
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

func (p *Pinecone) DeleteNamespace(ctx context.Context, namespace string) error {
	idxConn, err := p.indexConn(namespace)
	if err != nil {
		return err
	}

	err = idxConn.DeleteAllVectorsInNamespace(ctx)
	if err != nil {
		return fmt.Errorf("pinecone: delete namespace %q: %w", namespace, err)
	}
	return nil
}

func (p *Pinecone) Tables(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *Pinecone) CountVectors(ctx context.Context, namespace string) (int64, error) {
	if p.index == nil {
		return 0, fmt.Errorf("pinecone: not connected")
	}
	stats, err := p.index.DescribeIndexStats(ctx)
	if err != nil {
		return 0, fmt.Errorf("pinecone: describe index stats: %w", err)
	}
	if ns, ok := stats.Namespaces[namespace]; ok {
		return int64(ns.VectorCount), nil
	}
	return 0, nil
}

func (p *Pinecone) TotalVectors(ctx context.Context) (int64, error) {
	if p.index == nil {
		return 0, fmt.Errorf("pinecone: not connected")
	}

	stats, err := p.index.DescribeIndexStats(ctx)
	if err != nil {
		return 0, fmt.Errorf("pinecone: describe index stats: %w", err)
	}

	var total int64
	for _, ns := range stats.Namespaces {
		total += int64(ns.VectorCount)
	}
	return total, nil
}

// indexConn returns an IndexConnection for the given namespace.
// If namespace is empty, it returns the default connection.
func (p *Pinecone) indexConn(namespace string) (*pinecone.IndexConnection, error) {
	if p.client == nil {
		return nil, fmt.Errorf("pinecone: not connected")
	}
	if namespace == "" {
		return p.index, nil
	}
	return p.client.Index(pinecone.NewIndexConnParams{
		Host:      p.host,
		Namespace: namespace,
	})
}
