package reranker

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReranker_EmptyProvider_ReturnsNoop(t *testing.T) {
	cfg := &config.Config{RerankProvider: ""}
	r, err := NewReranker(cfg, nil)
	require.NoError(t, err)
	_, ok := r.(*NoopReranker)
	assert.True(t, ok)
}

func TestNewReranker_NonePrefix_ReturnsNoop(t *testing.T) {
	cfg := &config.Config{RerankProvider: "none"}
	r, err := NewReranker(cfg, nil)
	require.NoError(t, err)
	_, ok := r.(*NoopReranker)
	assert.True(t, ok)

	cfg.RerankProvider = "noop"
	r, err = NewReranker(cfg, nil)
	require.NoError(t, err)
	_, ok = r.(*NoopReranker)
	assert.True(t, ok)
}

func TestNewReranker_Cohere_BuildsPantheonReranker(t *testing.T) {
	cfg := &config.Config{
		RerankProvider:  "cohere",
		RerankAPIKey:    "key",
		RerankModelPref: "rerank-model",
	}
	r, err := NewReranker(cfg, nil)
	require.NoError(t, err)
	_, ok := r.(*PantheonReranker)
	assert.True(t, ok)
}

func TestNewReranker_UnknownProvider_ReturnsError(t *testing.T) {
	cfg := &config.Config{RerankProvider: "unknown"}
	_, err := NewReranker(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown rerank provider")
}

func TestNewReranker_CohereWithoutKey_ReturnsError(t *testing.T) {
	cfg := &config.Config{
		RerankProvider: "cohere",
		RerankAPIKey:   "",
		CohereApiKey:   "",
	}
	_, err := NewReranker(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no API key")
}
