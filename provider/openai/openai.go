// provider/openai/openai.go
package openai

import (
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/openaicompat"
)

const defaultBaseURL = "https://api.openai.com/v1"

// New constructs an OpenAI provider. Returns an error if the API key is missing.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("openai", defaultBaseURL, cfg, nil)
}
