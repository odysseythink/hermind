package embedder

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/nlpodyssey/cybertron/pkg/models/bert"
	"github.com/nlpodyssey/cybertron/pkg/tasks"
	"github.com/nlpodyssey/cybertron/pkg/tasks/textencoding"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
)

// NativeEmbedder implements Embedder using cybertron for local CPU embedding.
type NativeEmbedder struct {
	modelInfo NativeModelInfo
	cacheDir  string
	model     textencoding.Interface
	once      sync.Once
	initErr   error
	dims      int
}

// NewNativeEmbedder creates a new native embedder.
func NewNativeEmbedder(cfg *config.Config) (*NativeEmbedder, error) {
	modelID := cfg.NativeEmbeddingModel
	if modelID == "" {
		modelID = "sentence-transformers/all-MiniLM-L6-v2"
	}

	info, ok := getNativeModelInfo(modelID)
	if !ok {
		return nil, fmt.Errorf("native embedder: unsupported model %q", modelID)
	}

	cacheDir := filepath.Join(cfg.StorageDir, "models")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("native embedder: create cache dir: %w", err)
	}

	return &NativeEmbedder{
		modelInfo: info,
		cacheDir:  cacheDir,
		dims:      info.Dimensions,
	}, nil
}

// initModel lazily loads the cybertron model (thread-safe via sync.Once).
func (e *NativeEmbedder) initModel() error {
	e.once.Do(func() {
		mlog.Info("native embedder: loading model", mlog.String("model", e.modelInfo.HFRepo))

		m, err := tasks.Load[textencoding.Interface](&tasks.Config{
			ModelsDir:           e.cacheDir,
			ModelName:           e.modelInfo.HFRepo,
			DownloadPolicy:      tasks.DownloadMissing,
			ConversionPolicy:    tasks.ConvertMissing,
			ConversionPrecision: tasks.F32,
		})
		if err != nil {
			e.initErr = fmt.Errorf("native embedder: load model %s: %w", e.modelInfo.HFRepo, err)
			return
		}

		e.model = m
		mlog.Info("native embedder: model loaded", mlog.String("model", e.modelInfo.HFRepo))
	})
	return e.initErr
}

// EmbedTexts embeds multiple texts in batches.
func (e *NativeEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	if err := e.initModel(); err != nil {
		return nil, err
	}

	// Apply chunk prefix and truncate
	prepared := make([]string, len(texts))
	for i, text := range texts {
		t := e.modelInfo.ChunkPrefix + text
		if len(t) > e.modelInfo.EmbeddingMaxChunkLength {
			t = t[:e.modelInfo.EmbeddingMaxChunkLength]
			mlog.Warning("native embedder: truncated text exceeding max chunk length",
				mlog.Int("index", i),
				mlog.Int("original_len", len(text)),
				mlog.Int("max_len", e.modelInfo.EmbeddingMaxChunkLength))
		}
		prepared[i] = t
	}

	// Process in batches to limit memory usage
	maxBatch := e.modelInfo.MaxConcurrentChunks
	if maxBatch <= 0 {
		maxBatch = 25
	}

	results := make([][]float32, 0, len(prepared))
	for i := 0; i < len(prepared); i += maxBatch {
		end := i + maxBatch
		if end > len(prepared) {
			end = len(prepared)
		}
		batch := prepared[i:end]

		batchResults, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("native embedder: embed batch %d-%d: %w", i, end, err)
		}
		results = append(results, batchResults...)
	}

	return results, nil
}

// embedBatch embeds a single batch of texts.
func (e *NativeEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		result, err := e.model.Encode(ctx, text, int(bert.MeanPooling))
		if err != nil {
			return nil, fmt.Errorf("encode text %d: %w", i, err)
		}

		vec64 := result.Vector.Data().F64()
		vec32 := make([]float32, len(vec64))
		for j, v := range vec64 {
			vec32[j] = float32(v)
		}

		// L2 normalize to match anything-llm's normalize: true
		vec32 = l2Normalize(vec32)
		results[i] = vec32
	}
	return results, nil
}

// EmbedQuery embeds a single query text with query prefix.
func (e *NativeEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	query = e.modelInfo.QueryPrefix + query
	embeddings, err := e.EmbedTexts(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("native embedder: no embedding returned for query")
	}
	return embeddings[0], nil
}

// Dimensions returns the embedding vector size.
func (e *NativeEmbedder) Dimensions() int {
	return e.dims
}

// l2Normalize normalizes a vector to unit length.
func l2Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := math.Sqrt(sum)
	if norm == 0 {
		return v
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}
