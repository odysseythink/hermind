package providers

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "a", firstNonEmpty("", "a", "b"))
	assert.Equal(t, "", firstNonEmpty("", ""))
	assert.Equal(t, "x", firstNonEmpty("x"))
}

func TestResolveProviderName(t *testing.T) {
	cfg := &config.Config{LLMProvider: "openai"}
	assert.Equal(t, "openai", resolveProviderName(cfg, map[string]string{}))
	assert.Equal(t, "anthropic", resolveProviderName(cfg, map[string]string{"LLMProvider": "anthropic"}))
}

func TestResolveModelID(t *testing.T) {
	cfg := &config.Config{LLMModel: "fallback-model"}
	settings := map[string]string{"OpenAiModelPref": "gpt-4o"}
	assert.Equal(t, "gpt-4o", ResolveModelID("openai", cfg, settings))

	// fallback to cfg.LLMModel
	assert.Equal(t, "fallback-model", ResolveModelID("mistral", cfg, map[string]string{}))

	// default
	cfg2 := &config.Config{}
	assert.Equal(t, "gpt-4o-mini", ResolveModelID("openai", cfg2, map[string]string{}))
	assert.Equal(t, "claude-3-5-sonnet-20241022", ResolveModelID("anthropic", cfg2, map[string]string{}))
}

func TestResolveAPIKey_UnifiedPriority(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "openai",
		OpenAiKey:   "env-openai-key",
		LLMApiKey:   "env-generic-key",
	}

	// DB provider-specific key priority highest
	settings := map[string]string{"OpenAiKey": "db-openai-key"}
	key, err := ResolveAPIKey("openai", settings, cfg)
	require.NoError(t, err)
	assert.Equal(t, "db-openai-key", key)

	// env provider-specific key second
	key, err = ResolveAPIKey("openai", map[string]string{}, cfg)
	require.NoError(t, err)
	assert.Equal(t, "env-openai-key", key)

	// DB generic key third
	key, err = ResolveAPIKey("openai", map[string]string{"LLMApiKey": "db-generic-key"}, &config.Config{LLMProvider: "openai"})
	require.NoError(t, err)
	assert.Equal(t, "db-generic-key", key)

	// env generic key last fallback
	key, err = ResolveAPIKey("openai", map[string]string{}, &config.Config{LLMProvider: "openai", LLMApiKey: "env-generic-key"})
	require.NoError(t, err)
	assert.Equal(t, "env-generic-key", key)

	// all empty returns error
	_, err = ResolveAPIKey("openai", map[string]string{}, &config.Config{LLMProvider: "openai"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no API key configured")
}

func TestParseAzureEndpoint(t *testing.T) {
	rn, dep, err := parseAzureEndpoint("https://myresource.openai.azure.com/openai/deployments/mydeployment")
	assert.NoError(t, err)
	assert.Equal(t, "myresource", rn)
	assert.Equal(t, "mydeployment", dep)

	_, _, err = parseAzureEndpoint("not-a-url")
	assert.Error(t, err)

	_, _, err = parseAzureEndpoint("https://example.com/")
	assert.Error(t, err)
}
