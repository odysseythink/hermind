package factory_test

import (
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnthropic(t *testing.T) {
	p, err := factory.New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "sk-ant-test",
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", p.Name())
}

func TestNewOpenAI(t *testing.T) {
	p, err := factory.New(config.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-openai-test",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Name())
}

func TestNewDeepSeek(t *testing.T) {
	p, err := factory.New(config.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-deepseek-test",
		Model:    "deepseek-chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "deepseek", p.Name())
}

func TestNewAllChineseProviders(t *testing.T) {
	cases := []struct {
		name, provider, apiKey, model string
	}{
		{"qwen", "qwen", "sk-q", "qwen-max"},
		{"kimi", "kimi", "sk-k", "moonshot-v1-8k"},
		{"minimax", "minimax", "sk-m", "abab6.5s-chat"},
		{"zhipu", "zhipu", "id.secret", "glm-4"},
		{"wenxin", "wenxin", "api:secret", "ernie-speed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := factory.New(config.ProviderConfig{
				Provider: tc.provider,
				APIKey:   tc.apiKey,
				Model:    tc.model,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.name, p.Name())
		})
	}
}

func TestNewUnknown(t *testing.T) {
	_, err := factory.New(config.ProviderConfig{
		Provider: "gpt5-turbo-quantum",
		APIKey:   "sk-test",
	})
	assert.Error(t, err)
}

func TestNewAliases(t *testing.T) {
	// "glm" should map to zhipu
	p1, err := factory.New(config.ProviderConfig{
		Provider: "glm", APIKey: "id.secret", Model: "glm-4",
	})
	require.NoError(t, err)
	assert.Equal(t, "zhipu", p1.Name())

	// "moonshot" should map to kimi
	p2, err := factory.New(config.ProviderConfig{
		Provider: "moonshot", APIKey: "sk", Model: "moonshot-v1-8k",
	})
	require.NoError(t, err)
	assert.Equal(t, "kimi", p2.Name())

	// "ernie" should map to wenxin
	p3, err := factory.New(config.ProviderConfig{
		Provider: "ernie", APIKey: "api:secret", Model: "ernie-speed",
	})
	require.NoError(t, err)
	assert.Equal(t, "wenxin", p3.Name())
}
