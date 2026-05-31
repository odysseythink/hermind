package embedder

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/extensions/embed"
	"github.com/odysseythink/pantheon/providers/cohere"
	"github.com/odysseythink/pantheon/providers/openai"
	"github.com/odysseythink/pantheon/providers/voyage"
)

// NewEmbedder constructs an embedder based on cfg.EmbeddingEngine (or settings override).
func NewEmbedder(cfg *config.Config, settings map[string]string) (Embedder, error) {
	name := strings.ToLower(pickStr(settings, "EmbeddingEngine", cfg.EmbeddingEngine))
	apiKey := pickStr(settings, "EmbeddingApiKey", cfg.EmbeddingApiKey)
	if apiKey == "" {
		apiKey = pickStr(settings, "OpenAiKey", cfg.OpenAiKey)
	}
	baseURL := pickStr(settings, "EmbeddingBasePath", cfg.EmbeddingBasePath)
	modelID := pickStr(settings, "EmbeddingModel", cfg.EmbeddingModel)

	var prov core.Provider
	var err error
	switch name {
	case "cohere":
		if apiKey == "" {
			return nil, fmt.Errorf("cohere embedder: no API key")
		}
		prov, err = cohere.New(apiKey)
		if modelID == "" {
			modelID = "embed-english-v3.0"
		}
	case "voyage", "voyageai":
		if apiKey == "" {
			return nil, fmt.Errorf("voyage embedder: no API key")
		}
		prov, err = voyage.New(apiKey)
		if modelID == "" {
			modelID = "voyage-3"
		}
	case "native":
		return NewNativeEmbedder(cfg)
	default:
		// openai-compat: openai, ollama, lmstudio, localai, litellm, openrouter, azure, mistral, gemini, lemonade, genericopenai, etc.
		if apiKey == "" && requiresAPIKey(name) {
			return nil, fmt.Errorf("%s embedder: no API key", name)
		}
		opts := []openai.Option{}
		if baseURL != "" {
			baseURL = strings.TrimSuffix(baseURL, "/")
			baseURL = strings.TrimSuffix(baseURL, "/v1")
			opts = append(opts, openai.WithBaseURL(baseURL))
		}
		prov, err = openai.New(apiKey, opts...)
		if modelID == "" {
			modelID = defaultEmbeddingModelFor(name)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("create %s provider: %w", name, err)
	}

	embedProv, ok := prov.(embed.Provider)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support embedding", name)
	}
	model, err := embedProv.EmbeddingModel(context.Background(), modelID)
	if err != nil {
		return nil, fmt.Errorf("create embedding model: %w", err)
	}

	return &PantheonEmbedder{model: model}, nil
}

func pickStr(settings map[string]string, key, fallback string) string {
	if v, ok := settings[key]; ok && v != "" {
		return v
	}
	return fallback
}

func requiresAPIKey(name string) bool {
	switch name {
	case "ollama", "lmstudio", "localai", "litellm", "lemonade", "native":
		return false
	}
	return true
}

func defaultEmbeddingModelFor(name string) string {
	switch name {
	case "ollama":
		return "nomic-embed-text"
	case "lmstudio", "localai":
		return "nomic-embed-text-v1.5"
	case "gemini":
		return "text-embedding-004"
	case "mistral":
		return "mistral-embed"
	default:
		return "text-embedding-3-small"
	}
}
