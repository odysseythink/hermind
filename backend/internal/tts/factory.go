package tts

import (
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
)

// NewProvider returns a TTS Provider based on cfg/settings.
// Unknown or empty provider names fall back to the native stub.
func NewProvider(cfg *config.Config, settings map[string]string) Provider {
	name := strings.ToLower(strings.TrimSpace(pick("TTSProvider", settings, cfg.TTSProvider)))
	switch name {
	case "elevenlabs":
		return NewElevenLabsProvider(cfg, settings)
	case "openai":
		return NewOpenAIProvider(cfg, settings)
	case "openai-generic":
		return NewOpenAIGenericProvider(cfg, settings)
	default:
		return NewNativeProvider()
	}
}
