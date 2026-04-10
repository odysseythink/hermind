// provider/openai/openai.go
package openai

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

const defaultBaseURL = "https://api.openai.com/v1"

// New constructs an OpenAI provider. Returns an error if the API key is missing.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("openai", defaultBaseURL, cfg, nil)
}
