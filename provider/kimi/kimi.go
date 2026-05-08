// provider/kimi/kimi.go
package kimi

import (
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/openaicompat"
)

// Moonshot AI (月之暗面) hosts Kimi. The API is OpenAI-compatible.
const defaultBaseURL = "https://api.moonshot.cn/v1"

// New constructs a Kimi provider via Moonshot AI.
// Popular models: moonshot-v1-8k, moonshot-v1-32k, moonshot-v1-128k.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("kimi", defaultBaseURL, cfg, nil)
}
