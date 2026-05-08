// provider/openrouter/openrouter.go
package openrouter

import (
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/openaicompat"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

// New constructs an OpenRouter provider. OpenRouter is OpenAI-compatible
// but expects two extra headers for ranking/attribution:
//
//	HTTP-Referer: https://github.com/odysseythink/hermind
//	X-Title: hermind
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	headers := map[string]string{
		"HTTP-Referer": "https://github.com/odysseythink/hermind",
		"X-Title":      "hermind",
	}
	return openaicompat.NewFromProviderConfig("openrouter", defaultBaseURL, cfg, headers)
}
