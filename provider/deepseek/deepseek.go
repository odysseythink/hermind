// provider/deepseek/deepseek.go
package deepseek

import (
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/openaicompat"
)

const defaultBaseURL = "https://api.deepseek.com/v1"

// New constructs a DeepSeek provider. DeepSeek is OpenAI-compatible.
// Popular models: deepseek-chat, deepseek-reasoner (r1).
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("deepseek", defaultBaseURL, cfg, nil)
}
