package providers

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistry_Complete(t *testing.T) {
	expected := []string{
		"openai",
		"azure",
		"anthropic",
		"gemini",
		"lmstudio",
		"localai",
		"ollama",
		"togetherai",
		"fireworksai",
		"mistral",
		"huggingface",
		"perplexity",
		"openrouter",
		"novita",
		"groq",
		"koboldcpp",
		"textgenwebui",
		"cohere",
		"litellm",
		"generic-openai",
		"bedrock",
		"deepseek",
		"apipie",
		"xai",
		"nvidia-nim",
		"ppio",
		"dpaiStudio",
		"moonshotai",
		"cometapi",
		"foundry",
		"zai",
		"giteeai",
		"docker-model-runner",
		"privatemode",
		"sambanova",
		"lemonade",
		"minimax",
		"qwen",
		"wenxin",
		"zhipu",
	}

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			b, ok := providerRegistry[name]
			require.True(t, ok, "provider %s not in registry", name)
			require.NotNil(t, b)
		})
	}

	assert.Equal(t, len(expected), len(providerRegistry), "registry size mismatch: expected %d, got %d", len(expected), len(providerRegistry))
}

func TestBuilders_MissingConfig(t *testing.T) {
	cfg := &config.Config{}
	settings := map[string]string{}
	ctx := context.Background()

	// Standard API-key providers
	apiKeyProviders := []struct {
		name    string
		modelID string
	}{
		{"openai", "gpt-4o"},
		{"anthropic", "claude-3"},
		{"gemini", "gemini-pro"},
		{"togetherai", "llama-3"},
		{"fireworksai", "llama-3"},
		{"mistral", "mistral-large"},
		{"perplexity", "llama-3"},
		{"openrouter", "gpt-4o"},
		{"novita", "llama-3"},
		{"groq", "llama-3"},
		{"cohere", "command-r"},
		{"deepseek", "deepseek-chat"},
		{"apipie", "gpt-4o"},
		{"xai", "grok-beta"},
		{"nvidia-nim", "llama-3"},
		{"ppio", "llama-3"},
		{"moonshotai", "moonshot-v1"},
		{"cometapi", "gpt-4o"},
		{"zai", "gpt-4o"},
		{"giteeai", "gpt-4o"},
		{"sambanova", "llama-3"},
		{"minimax", "abab6"},
		{"qwen", "qwen-turbo"},
		{"wenxin", "ernie-bot"},
		{"zhipu", "glm-4"},
	}

	for _, tc := range apiKeyProviders {
		t.Run(tc.name+"_missing_api_key", func(t *testing.T) {
			builder, ok := providerRegistry[tc.name]
			require.True(t, ok, "provider %s not found in registry", tc.name)
			_, err := builder(ctx, cfg, settings, tc.modelID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "configured")
		})
	}

	// BaseURL-required providers
	baseURLProviders := []struct {
		name    string
		modelID string
	}{
		{"lmstudio", "local-model"},
		{"localai", "local-model"},
		{"koboldcpp", "local-model"},
		{"textgenwebui", "local-model"},
		{"huggingface", "tgi-model"},
		{"litellm", "local-model"},
		{"generic-openai", "local-model"},
		{"dpaiStudio", "local-model"},
		{"docker-model-runner", "local-model"},
		{"foundry", "gpt-4o"},
		{"nvidia-nim", "llama-3"},
		{"privatemode", "gpt-4o"},
		{"lemonade", "gpt-4o"},
	}

	for _, tc := range baseURLProviders {
		t.Run(tc.name+"_missing_base_url", func(t *testing.T) {
			builder, ok := providerRegistry[tc.name]
			require.True(t, ok, "provider %s not found in registry", tc.name)
			_, err := builder(ctx, cfg, settings, tc.modelID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "configured")
		})
	}
}

func TestBuildAzure_EndpointParsing(t *testing.T) {
	ctx := context.Background()

	t.Run("resource_and_deployment_from_endpoint", func(t *testing.T) {
		cfg := &config.Config{
			AzureOpenAiKey:      "test-key",
			AzureOpenAiEndpoint: "https://myresource.openai.azure.com/openai/deployments/mydeployment",
		}
		builder := providerRegistry["azure"]
		model, err := builder(ctx, cfg, map[string]string{}, "gpt-4o")
		// Should succeed past config check; LanguageModel creation doesn't need real Azure.
		require.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("explicit_fields_override_endpoint", func(t *testing.T) {
		cfg := &config.Config{
			AzureOpenAiKey:          "test-key",
			AzureOpenAiResourceName: "explicit-resource",
			AzureOpenAiDeployment:   "explicit-deployment",
		}
		builder := providerRegistry["azure"]
		model, err := builder(ctx, cfg, map[string]string{}, "gpt-4o")
		require.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("missing_all_config", func(t *testing.T) {
		cfg := &config.Config{}
		builder := providerRegistry["azure"]
		_, err := builder(ctx, cfg, map[string]string{}, "gpt-4o")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no API key configured for provider azure")
	})
}

func TestBuildOllama_DefaultBaseURL(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	builder := providerRegistry["ollama"]
	// Ollama has a default baseURL, so even with empty config it should succeed.
	model, err := builder(ctx, cfg, map[string]string{}, "llama3")
	require.NoError(t, err)
	assert.NotNil(t, model)
}

func TestBuildBedrock_MissingFields(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	t.Run("missing_all", func(t *testing.T) {
		builder := providerRegistry["bedrock"]
		_, err := builder(ctx, cfg, map[string]string{}, "claude-3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bedrock: accessKeyID, secretKey, and region are required")
	})

	t.Run("missing_region", func(t *testing.T) {
		cfg2 := &config.Config{
			AwsBedrockLLMAccessKeyId: "AKIAIOSFODNN7EXAMPLE",
			AwsBedrockLLMAccessKey:   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		builder := providerRegistry["bedrock"]
		_, err := builder(ctx, cfg2, map[string]string{}, "claude-3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bedrock: accessKeyID, secretKey, and region are required")
	})
}

func TestNewLLMProvider_UnsupportedProvider(t *testing.T) {
	cfg := &config.Config{LLMProvider: "nonexistent"}
	prov := NewLLMProvider(cfg, map[string]string{})
	require.NotNil(t, prov)
	_, err := prov.Stream(context.Background(), nil, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported LLM provider")
}

func TestBuildMinimax_ReturnsLM(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "fake-key")
	cfg, _ := config.Load()
	cfg.LLMProvider = "minimax"
	lm := NewLLMProvider(cfg, nil)
	_, isNoop := lm.(*noopLLM)
	require.False(t, isNoop)
}

func TestBuildQwen_ReturnsLM(t *testing.T) {
	t.Setenv("QWEN_API_KEY", "fake-key")
	cfg, _ := config.Load()
	cfg.LLMProvider = "qwen"
	lm := NewLLMProvider(cfg, nil)
	_, isNoop := lm.(*noopLLM)
	require.False(t, isNoop)
}

func TestBuildWenxin_ReturnsLM(t *testing.T) {
	t.Setenv("WENXIN_API_KEY", "fake-key")
	cfg, _ := config.Load()
	cfg.LLMProvider = "wenxin"
	lm := NewLLMProvider(cfg, nil)
	_, isNoop := lm.(*noopLLM)
	require.False(t, isNoop)
}


func TestBuildZhipu_ReturnsLM(t *testing.T) {
	t.Setenv("ZHIPU_API_KEY", "fake-key")
	cfg, _ := config.Load()
	cfg.LLMProvider = "zhipu"
	lm := NewLLMProvider(cfg, nil)
	_, isNoop := lm.(*noopLLM)
	require.False(t, isNoop)
}
