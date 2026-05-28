package providers

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "gpt-4o", resolveModelID("openai", cfg, settings))

	// fallback to cfg.LLMModel
	assert.Equal(t, "fallback-model", resolveModelID("mistral", cfg, map[string]string{}))

	// default
	cfg2 := &config.Config{}
	assert.Equal(t, "gpt-4o-mini", resolveModelID("openai", cfg2, map[string]string{}))
	assert.Equal(t, "claude-3-5-sonnet-20241022", resolveModelID("anthropic", cfg2, map[string]string{}))
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
