package services

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
)

type VectorService struct {
	cfg      *config.Config
	provider vectordb.VectorDatabase
}

func NewVectorService(cfg *config.Config) *VectorService {
	return &VectorService{cfg: cfg}
}

func (s *VectorService) Connect(ctx context.Context) error {
	var provider vectordb.VectorDatabase
	switch s.cfg.VectorDB {
	case "lancedb":
		provider = vectordb.NewLanceDB(s.cfg.StorageDir)
	case "pgvector":
		provider = vectordb.NewPGVector(s.cfg.DatabaseURL)
	case "pinecone":
		provider = vectordb.NewPinecone(s.cfg.PineconeAPIKey, s.cfg.PineconeIndex)
	case "qdrant":
		provider = vectordb.NewQdrant(s.cfg.QdrantEndpoint, s.cfg.QdrantAPIKey)
	case "chroma":
		provider = vectordb.NewChroma(s.cfg.ChromaEndpoint, s.cfg.ChromaAPIHeader, s.cfg.ChromaAPIKey)
	case "weaviate":
		provider = vectordb.NewWeaviate(s.cfg.WeaviateEndpoint, s.cfg.WeaviateAPIKey)
	case "milvus":
		provider = vectordb.NewMilvus(s.cfg.MilvusAddress, s.cfg.MilvusUsername, s.cfg.MilvusPassword)
	case "zilliz":
		provider = vectordb.NewZilliz(s.cfg.ZillizEndpoint, s.cfg.ZillizAPIToken)
	case "astra":
		provider = vectordb.NewAstraDB(s.cfg.AstraDBApplicationToken, s.cfg.AstraDBEndpoint)
	case "chromacloud":
		provider = vectordb.NewChromaCloud(s.cfg.ChromaEndpoint, s.cfg.ChromaAPIKey)
	default:
		return fmt.Errorf("unknown vector db: %s", s.cfg.VectorDB)
	}

	if err := provider.Connect(ctx); err != nil {
		return fmt.Errorf("connect %s: %w", s.cfg.VectorDB, err)
	}

	s.provider = provider
	return nil
}

func (s *VectorService) SetProvider(p vectordb.VectorDatabase) {
	s.provider = p
}

func (s *VectorService) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts vectordb.SearchOptions) ([]vectordb.SearchResult, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("vector provider not connected")
	}
	return s.provider.SimilaritySearch(ctx, namespace, queryVector, opts)
}

func (s *VectorService) AddVectors(ctx context.Context, namespace string, chunks []vectordb.VectorChunk) error {
	if s.provider == nil {
		return fmt.Errorf("vector provider not connected")
	}
	return s.provider.AddVectors(ctx, namespace, chunks)
}

func (s *VectorService) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if s.provider == nil {
		return fmt.Errorf("vector provider not connected")
	}
	return s.provider.DeleteVectors(ctx, namespace, vectorIds)
}

func (s *VectorService) Heartbeat(ctx context.Context) (map[string]any, error) {
	if s.provider == nil {
		return map[string]any{"status": "not configured"}, nil
	}
	return s.provider.Heartbeat(ctx)
}

func (s *VectorService) CountVectors(ctx context.Context, namespace string) (int64, error) {
	if s.provider == nil {
		return 0, fmt.Errorf("vector provider not connected")
	}
	return s.provider.CountVectors(ctx, namespace)
}

func (s *VectorService) TotalVectors(ctx context.Context) (int64, error) {
	if s.provider == nil {
		return 0, fmt.Errorf("vector provider not connected")
	}
	return s.provider.TotalVectors(ctx)
}
