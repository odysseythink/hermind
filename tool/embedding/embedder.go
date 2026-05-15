// tool/embedding/embedder.go
package embedding

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"

	"github.com/odysseythink/hermind/provider"
)

// ErrNotSupported is returned when the underlying provider does not
// implement provider.EmbedCapable.
var ErrNotSupported = errors.New("embedding: provider does not support embeddings")

// Embedder generates a float32 vector for a text string.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// ProviderEmbedder wraps a provider.EmbedCapable.
type ProviderEmbedder struct {
	p     provider.EmbedCapable
	model string // embedding model, e.g. "text-embedding-3-small"
}

// NewProviderEmbedder constructs a ProviderEmbedder.
// model is the embedding model name (provider-specific).
func NewProviderEmbedder(p provider.EmbedCapable, model string) *ProviderEmbedder {
	return &ProviderEmbedder{p: p, model: model}
}

func (pe *ProviderEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if pe.p == nil {
		return nil, ErrNotSupported
	}
	return pe.p.Embed(ctx, pe.model, text)
}

// EncodeVector gob-encodes a []float32 into bytes for SQLite BLOB storage.
func EncodeVector(v []float32) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeVector decodes a gob-encoded []float32 from SQLite BLOB bytes.
func DecodeVector(b []byte) ([]float32, error) {
	var v []float32
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}
