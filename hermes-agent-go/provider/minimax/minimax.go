// provider/minimax/minimax.go
package minimax

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

// MiniMax OpenAI-compatible endpoint.
const defaultBaseURL = "https://api.minimax.chat/v1"

// New constructs a MiniMax provider.
// Popular models: abab6.5s-chat, abab6.5t-chat, MiniMax-Text-01.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("minimax", defaultBaseURL, cfg, nil)
}
