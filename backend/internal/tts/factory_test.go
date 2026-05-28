package tts

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/require"
)

func TestFactory_ElevenLabs_BuildsProvider(t *testing.T) {
	cfg := &config.Config{
		TTSProvider:       "elevenlabs",
		ElevenLabsAPIKey:  "key",
		ElevenLabsVoiceID: "vid",
	}
	p := NewProvider(cfg, nil)
	require.Equal(t, "elevenlabs", p.Name())
	require.True(t, p.Available())
}

func TestFactory_OpenAI_BuildsProvider(t *testing.T) {
	cfg := &config.Config{
		TTSProvider: "openai",
		OpenAiKey:   "sk-test",
	}
	p := NewProvider(cfg, nil)
	require.Equal(t, "openai", p.Name())
	require.True(t, p.Available())
}

func TestFactory_OpenAIGeneric_BuildsProvider(t *testing.T) {
	cfg := &config.Config{
		TTSProvider:             "openai-generic",
		TTSOpenAICompatEndpoint: "http://localhost:1234",
	}
	p := NewProvider(cfg, nil)
	require.Equal(t, "openai-generic", p.Name())
	require.True(t, p.Available())
}

func TestFactory_Native_DefaultForUnknown(t *testing.T) {
	cfg := &config.Config{TTSProvider: "unknown-provider"}
	p := NewProvider(cfg, nil)
	require.Equal(t, "native", p.Name())
	require.True(t, p.Available())
}

func TestFactory_EmptyConfig_FallsBackToNative(t *testing.T) {
	cfg := &config.Config{}
	p := NewProvider(cfg, nil)
	require.Equal(t, "native", p.Name())
}

func TestFactory_SettingsOverridesCfg(t *testing.T) {
	cfg := &config.Config{
		TTSProvider:       "openai",
		OpenAiKey:         "sk-test",
		ElevenLabsAPIKey:  "key",
		ElevenLabsVoiceID: "vid",
	}
	settings := map[string]string{"TTSProvider": "elevenlabs"}
	p := NewProvider(cfg, settings)
	require.Equal(t, "elevenlabs", p.Name())
}
