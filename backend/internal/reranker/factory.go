package reranker

import (
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/pantheon/providers/openaicompat"
)

// NewReranker returns a Reranker selected by cfg.RerankProvider.
func NewReranker(cfg *config.Config, settings map[string]string) (Reranker, error) {
	name := strings.ToLower(pickStr(settings, "RerankProvider", cfg.RerankProvider))
	switch name {
	case "", "none", "noop":
		return &NoopReranker{}, nil
	case "cohere":
		apiKey := pickStr(settings, "RerankApiKey", cfg.RerankAPIKey)
		if apiKey == "" {
			apiKey = pickStr(settings, "CohereApiKey", cfg.CohereApiKey)
		}
		if apiKey == "" {
			return nil, fmt.Errorf("cohere reranker: no API key")
		}
		client := openaicompat.NewClient("https://api.cohere.com", apiKey)
		client.RerankPath = "/v2/rerank"
		client.RerankFormat = openaicompat.RerankFormatCohereV2
		modelID := pickStr(settings, "RerankModelPref", cfg.RerankModelPref)
		if modelID == "" {
			modelID = "rerank-english-v3.0"
		}
		return NewPantheonReranker(&openAICompatRerankModel{client: client, model: modelID}), nil
	default:
		return nil, fmt.Errorf("unknown rerank provider: %s", name)
	}
}

func pickStr(settings map[string]string, key, fallback string) string {
	if v, ok := settings[key]; ok && v != "" {
		return v
	}
	return fallback
}
