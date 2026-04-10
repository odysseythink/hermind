// provider/openrouter/openrouter.go
package openrouter

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

// New constructs an OpenRouter provider. OpenRouter is OpenAI-compatible
// but expects two extra headers for ranking/attribution:
//
//	HTTP-Referer: https://github.com/nousresearch/hermes-agent
//	X-Title: hermes-agent
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	headers := map[string]string{
		"HTTP-Referer": "https://github.com/nousresearch/hermes-agent",
		"X-Title":      "hermes-agent",
	}
	return openaicompat.NewFromProviderConfig("openrouter", defaultBaseURL, cfg, headers)
}
