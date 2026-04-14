// provider/minimax/minimax.go
package minimax

import (
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/openaicompat"
)

// MiniMax OpenAI-compatible endpoint.
const defaultBaseURL = "https://api.minimax.chat/v1"

// New constructs a MiniMax provider.
// Popular models: abab6.5s-chat, abab6.5t-chat, MiniMax-Text-01.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("minimax", defaultBaseURL, cfg, nil)
}
