package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/providers/ollama"
	"github.com/odysseythink/pantheon/providers/openai"
)

// BuildLanguageModelForTesting exposes buildLanguageModel for unit tests.
func BuildLanguageModelForTesting(ws *models.Workspace, settings map[string]string, cfg *config.Config) (core.LanguageModel, error) {
	return buildLanguageModel(ws, settings, cfg)
}

func buildLanguageModel(ws *models.Workspace, settings map[string]string, cfg *config.Config) (core.LanguageModel, error) {
	providerName := pick("LLMProvider", settings, cfg.LLMProvider)
	modelID := providers.ResolveModelID(providerName, cfg, settings)

	switch providerName {
	case "ollama":
		baseURL := strings.TrimSuffix(strings.TrimSuffix(pick("OllamaLLMBasePath", settings, "http://127.0.0.1:11434"), "/"), "/v1")
		p, err := ollama.New("", ollama.WithBaseURL(baseURL))
		if err != nil {
			return nil, fmt.Errorf("ollama provider: %w", err)
		}
		return p.LanguageModel(context.Background(), modelID)
	default:
		apiKey, err := providers.ResolveAPIKey(providerName, settings, cfg)
		if err != nil {
			return nil, fmt.Errorf("no LLM API key configured for provider %q: %w", providerName, err)
		}
		p, err := openai.New(apiKey)
		if err != nil {
			return nil, fmt.Errorf("openai provider: %w", err)
		}
		return p.LanguageModel(context.Background(), modelID)
	}
}

func pick(key string, settings map[string]string, fallback string) string {
	if v, ok := settings[key]; ok && v != "" {
		return v
	}
	return fallback
}
