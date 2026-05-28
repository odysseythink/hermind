package embedder

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmbedder_OpenAI_BuildsModel(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "openai")
	t.Setenv("EMBEDDING_API_KEY", "sk-test")

	cfg, err := config.Load()
	require.NoError(t, err)

	emb, err := NewEmbedder(cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, emb)
	_, ok := emb.(*PantheonEmbedder)
	assert.True(t, ok)
}

func TestNewEmbedder_OpenAICompatViaOllama_BuildsModel(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "ollama")
	t.Setenv("EMBEDDING_BASE_PATH", "http://localhost:11434")

	cfg, err := config.Load()
	require.NoError(t, err)

	emb, err := NewEmbedder(cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, emb)
	_, ok := emb.(*PantheonEmbedder)
	assert.True(t, ok)
}

func TestNewEmbedder_Cohere_BuildsModel(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "cohere")
	t.Setenv("EMBEDDING_API_KEY", "sk-test")

	cfg, err := config.Load()
	require.NoError(t, err)

	emb, err := NewEmbedder(cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, emb)
	_, ok := emb.(*PantheonEmbedder)
	assert.True(t, ok)
}

func TestNewEmbedder_Voyage_BuildsModel(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "voyage")
	t.Setenv("EMBEDDING_API_KEY", "sk-test")

	cfg, err := config.Load()
	require.NoError(t, err)

	emb, err := NewEmbedder(cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, emb)
	_, ok := emb.(*PantheonEmbedder)
	assert.True(t, ok)
}

func TestNewEmbedder_NoAPIKey_ReturnsError(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "openai")
	t.Setenv("EMBEDDING_API_KEY", "")
	t.Setenv("OPEN_AI_KEY", "")

	cfg, err := config.Load()
	require.NoError(t, err)

	_, err = NewEmbedder(cfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no API key")
}

func TestNewEmbedder_UnknownProvider_FallsBackToOpenAICompat(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "some-unknown-provider")
	t.Setenv("EMBEDDING_API_KEY", "sk-test")

	cfg, err := config.Load()
	require.NoError(t, err)

	emb, err := NewEmbedder(cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, emb)
	_, ok := emb.(*PantheonEmbedder)
	assert.True(t, ok)
}

func TestNewEmbedder_LMStudio_NoKeyRequired(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "lmstudio")
	t.Setenv("EMBEDDING_BASE_PATH", "http://localhost:1234")

	cfg, err := config.Load()
	require.NoError(t, err)

	emb, err := NewEmbedder(cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, emb)
	_, ok := emb.(*PantheonEmbedder)
	assert.True(t, ok)
}

func TestNewPantheonEmbedder_DeprecatedStillWorks(t *testing.T) {
	t.Setenv("EMBEDDING_ENGINE", "openai")
	t.Setenv("EMBEDDING_API_KEY", "sk-test")

	cfg, err := config.Load()
	require.NoError(t, err)

	emb, err := NewPantheonEmbedder(cfg)
	require.NoError(t, err)
	assert.NotNil(t, emb)
}
