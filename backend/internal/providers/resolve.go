package providers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
)

// ResolveAPIKey returns the API key for a provider following the unified priority:
//
//	settings[providerSpecificKey] > cfg.ProviderSpecificKey > settings["LLMApiKey"] > cfg.LLMApiKey
func ResolveAPIKey(provider string, settings map[string]string, cfg *config.Config) (string, error) {
	if specificField := providerAPIKeyField(provider); specificField != "" {
		if v, ok := settings[specificField]; ok && v != "" {
			return v, nil
		}
		if v := cfgAPIKeyByField(cfg, specificField); v != "" {
			return v, nil
		}
	}
	if v, ok := settings["LLMApiKey"]; ok && v != "" {
		return v, nil
	}
	if cfg.LLMApiKey != "" {
		return cfg.LLMApiKey, nil
	}
	return "", fmt.Errorf("no API key configured for provider %s", provider)
}

// ResolveModelID is the exported version of resolveModelID for chat / agent paths.
func ResolveModelID(provider string, cfg *config.Config, settings map[string]string) string {
	return resolveModelID(provider, cfg, settings)
}

// providerAPIKeyField maps a provider name to the corresponding Config field name for its API key.
func providerAPIKeyField(provider string) string {
	switch provider {
	case "openai":
		return "OpenAiKey"
	case "azure":
		return "AzureOpenAiKey"
	case "anthropic":
		return "AnthropicApiKey"
	case "gemini":
		return "GeminiLLMApiKey"
	case "localai":
		return "LocalAiApiKey"
	case "togetherai":
		return "TogetherAiApiKey"
	case "fireworksai":
		return "FireworksApiKey"
	case "mistral":
		return "MistralApiKey"
	case "huggingface":
		return "HuggingFaceLLMAccessToken"
	case "perplexity":
		return "PerplexityApiKey"
	case "openrouter":
		return "OpenRouterApiKey"
	case "novita":
		return "NovitaLLMApiKey"
	case "groq":
		return "GroqApiKey"
	case "koboldcpp":
		return "KoboldCPPApiKey"
	case "textgenwebui":
		return "TextGenWebUIAPIKey"
	case "cohere":
		return "CohereApiKey"
	case "litellm":
		return "LiteLLMApiKey"
	case "generic-openai":
		return "GenericOpenAiKey"
	case "deepseek":
		return "DeepSeekApiKey"
	case "apipie":
		return "ApiPieApiKey"
	case "xai":
		return "XAIApiKey"
	case "ppio":
		return "PpioApiKey"
	case "dpaiStudio":
		return "DellProApiKey"
	case "moonshotai":
		return "MoonshotAiApiKey"
	case "cometapi":
		return "CometApiLLMApiKey"
	case "zai":
		return "ZAiApiKey"
	case "giteeai":
		return "GiteeAIApiKey"
	case "docker-model-runner":
		return "DockerModelRunnerApiKey"
	case "privatemode":
		return "PrivateModeApiKey"
	case "sambanova":
		return "SambaNovaLLMApiKey"
	case "lemonade":
		return "LemonadeLLMApiKey"
	case "minimax":
		return "MinimaxApiKey"
	case "qwen":
		return "QwenApiKey"
	case "wenxin":
		return "WenxinApiKey"
	case "zhipu":
		return "ZhipuApiKey"
	}
	return ""
}

// cfgAPIKeyByField reads a Config API key by field name.
func cfgAPIKeyByField(cfg *config.Config, field string) string {
	switch field {
	case "OpenAiKey":
		return cfg.OpenAiKey
	case "AzureOpenAiKey":
		return cfg.AzureOpenAiKey
	case "AnthropicApiKey":
		return cfg.AnthropicApiKey
	case "GeminiLLMApiKey":
		return cfg.GeminiApiKey
	case "LocalAiApiKey":
		return cfg.LocalAiApiKey
	case "TogetherAiApiKey":
		return cfg.TogetherAiApiKey
	case "FireworksApiKey":
		return cfg.FireworksApiKey
	case "MistralApiKey":
		return cfg.MistralApiKey
	case "HuggingFaceLLMAccessToken":
		return cfg.HuggingFaceApiKey
	case "PerplexityApiKey":
		return cfg.PerplexityApiKey
	case "OpenRouterApiKey":
		return cfg.OpenRouterApiKey
	case "NovitaLLMApiKey":
		return cfg.NovitaLLMApiKey
	case "GroqApiKey":
		return cfg.GroqApiKey
	case "KoboldCPPApiKey":
		return cfg.LLMApiKey
	case "TextGenWebUIAPIKey":
		return cfg.TextGenApiKey
	case "CohereApiKey":
		return cfg.CohereApiKey
	case "LiteLLMApiKey":
		return cfg.LiteLLMApiKey
	case "GenericOpenAiKey":
		return cfg.GenericOpenAiKey
	case "DeepSeekApiKey":
		return cfg.DeepSeekApiKey
	case "ApiPieApiKey":
		return cfg.ApiPieApiKey
	case "XAIApiKey":
		return cfg.XAIApiKey
	case "PpioApiKey":
		return cfg.PpioApiKey
	case "DellProApiKey":
		return cfg.LLMApiKey
	case "MoonshotAiApiKey":
		return cfg.MoonshotAiApiKey
	case "CometApiLLMApiKey":
		return cfg.CometApiLLMApiKey
	case "ZAiApiKey":
		return cfg.ZAiApiKey
	case "GiteeAIApiKey":
		return cfg.GiteeAIApiKey
	case "DockerModelRunnerApiKey":
		return cfg.LLMApiKey
	case "PrivateModeApiKey":
		return cfg.LLMApiKey
	case "SambaNovaLLMApiKey":
		return cfg.SambaNovaLLMApiKey
	case "LemonadeLLMApiKey":
		return cfg.LemonadeLLMApiKey
	case "MinimaxApiKey":
		return cfg.MinimaxAPIKey
	case "QwenApiKey":
		return cfg.QwenAPIKey
	case "WenxinApiKey":
		return cfg.WenxinAPIKey
	case "ZhipuApiKey":
		return cfg.ZhipuAPIKey
	}
	return ""
}

// maskKey masks an API key for logging: first 7 chars + "..." + last 4 chars.
func maskKey(key string) string {
	if len(key) <= 11 {
		return strings.Repeat("*", len(key))
	}
	return key[:7] + "..." + key[len(key)-4:]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveProviderName(cfg *config.Config, settings map[string]string) string {
	if v, ok := settings["LLMProvider"]; ok && v != "" {
		return v
	}
	return cfg.LLMProvider
}

func resolveModelID(provider string, cfg *config.Config, settings map[string]string) string {
	key := modelPrefKeyForProvider(provider)
	if v, ok := settings[key]; ok && v != "" {
		return v
	}
	if v := cfgModelPref(cfg, provider); v != "" {
		return v
	}
	if cfg.LLMModel != "" {
		return cfg.LLMModel
	}
	return defaultModelForProvider(provider)
}

func modelPrefKeyForProvider(provider string) string {
	switch provider {
	case "openai":
		return "OpenAiModelPref"
	case "azure":
		return "AzureOpenAiModelPref"
	case "anthropic":
		return "AnthropicModelPref"
	case "gemini":
		return "GeminiLLMModelPref"
	case "lmstudio":
		return "LMStudioModelPref"
	case "localai":
		return "LocalAiModelPref"
	case "ollama":
		return "OllamaLLMModelPref"
	case "togetherai":
		return "TogetherAiModelPref"
	case "fireworksai":
		return "FireworksModelPref"
	case "mistral":
		return "MistralModelPref"
	case "huggingface":
		return "HuggingFaceLLMModelPref"
	case "perplexity":
		return "PerplexityModelPref"
	case "openrouter":
		return "OpenRouterModelPref"
	case "novita":
		return "NovitaModelPref"
	case "groq":
		return "GroqModelPref"
	case "koboldcpp":
		return "KoboldCPPModelPref"
	case "textgenwebui":
		return "TextGenWebUIModelPref"
	case "cohere":
		return "CohereModelPref"
	case "litellm":
		return "LiteLLMModelPref"
	case "generic-openai":
		return "GenericOpenAiModelPref"
	case "bedrock":
		return "AwsBedrockLLMModel"
	case "deepseek":
		return "DeepSeekModelPref"
	case "apipie":
		return "ApiPieModelPref"
	case "xai":
		return "XAIModelPref"
	case "nvidia-nim":
		return "NvidiaNimLLMModelPref"
	case "ppio":
		return "PpioModelPref"
	case "dpaiStudio":
		return "DellProAiStudioModelPref"
	case "moonshotai":
		return "MoonshotAiModelPref"
	case "cometapi":
		return "CometApiLLMModelPref"
	case "foundry":
		return "FoundryModelPref"
	case "zai":
		return "ZAiModelPref"
	case "giteeai":
		return "GiteeAIModelPref"
	case "docker-model-runner":
		return "DockerModelRunnerModelPref"
	case "privatemode":
		return "PrivateModeModelPref"
	case "sambanova":
		return "SambaNovaLLMModelPref"
	case "lemonade":
		return "LemonadeLLMModelPref"
	case "minimax":
		return "MinimaxModelPref"
	case "qwen":
		return "QwenModelPref"
	case "wenxin":
		return "WenxinModelPref"
	case "zhipu":
		return "ZhipuModelPref"
	}
	return ""
}

func cfgModelPref(cfg *config.Config, provider string) string {
	switch provider {
	case "openai":
		return cfg.OpenAiModelPref
	case "azure":
		return cfg.AzureOpenAiModelPref
	case "anthropic":
		return cfg.AnthropicModelPref
	case "gemini":
		return cfg.GeminiModelPref
	case "lmstudio":
		return cfg.LMStudioModelPref
	case "localai":
		return cfg.LocalAiModelPref
	case "ollama":
		return cfg.OllamaModelPref
	case "togetherai":
		return cfg.TogetherAiModelPref
	case "fireworksai":
		return cfg.FireworksModelPref
	case "mistral":
		return cfg.MistralModelPref
	case "huggingface":
		return ""
	case "perplexity":
		return cfg.PerplexityModelPref
	case "openrouter":
		return cfg.OpenRouterModelPref
	case "novita":
		return cfg.NovitaLLMModelPref
	case "groq":
		return cfg.GroqModelPref
	case "koboldcpp":
		return cfg.KoboldModelPref
	case "textgenwebui":
		return ""
	case "cohere":
		return cfg.CohereModelPref
	case "litellm":
		return cfg.LiteLLMModelPref
	case "generic-openai":
		return cfg.GenericOpenAiKey
	case "bedrock":
		return cfg.AwsBedrockLLMModel
	case "deepseek":
		return cfg.DeepSeekModelPref
	case "apipie":
		return cfg.ApiPieModelPref
	case "xai":
		return cfg.XAIModelPref
	case "nvidia-nim":
		return cfg.NvidiaNimLLMModelPref
	case "ppio":
		return cfg.PpioModelPref
	case "dpaiStudio":
		return cfg.DellProAiStudioModelPref
	case "moonshotai":
		return cfg.MoonshotAiModelPref
	case "cometapi":
		return cfg.CometApiLLMModelPref
	case "foundry":
		return cfg.FoundryModelPref
	case "zai":
		return cfg.ZAiModelPref
	case "giteeai":
		return cfg.GiteeAIModelPref
	case "docker-model-runner":
		return cfg.DockerModelRunnerModelPref
	case "privatemode":
		return cfg.PrivateModeModelPref
	case "sambanova":
		return cfg.SambaNovaLLMModelPref
	case "lemonade":
		return cfg.LemonadeLLMModelPref
	case "minimax":
		return cfg.MinimaxModelPref
	case "qwen":
		return cfg.QwenModelPref
	case "wenxin":
		return cfg.WenxinModelPref
	case "zhipu":
		return cfg.ZhipuModelPref
	}
	return ""
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o-mini"
	case "azure":
		return "gpt-4o"
	case "anthropic":
		return "claude-3-5-sonnet-20241022"
	case "gemini":
		return "gemini-1.5-flash"
	case "lmstudio":
		return ""
	case "localai":
		return ""
	case "ollama":
		return ""
	case "togetherai":
		return ""
	case "fireworksai":
		return ""
	case "mistral":
		return "mistral-large-latest"
	case "huggingface":
		return ""
	case "perplexity":
		return "llama-3.1-sonar-small-128k-online"
	case "openrouter":
		return ""
	case "novita":
		return ""
	case "groq":
		return "llama3-8b-8192"
	case "koboldcpp":
		return ""
	case "textgenwebui":
		return ""
	case "cohere":
		return "command-r"
	case "litellm":
		return ""
	case "generic-openai":
		return ""
	case "bedrock":
		return ""
	case "deepseek":
		return "deepseek-chat"
	case "apipie":
		return ""
	case "xai":
		return "grok-beta"
	case "nvidia-nim":
		return ""
	case "ppio":
		return ""
	case "dpaiStudio":
		return ""
	case "moonshotai":
		return "moonshot-v1-8k"
	case "cometapi":
		return ""
	case "foundry":
		return ""
	case "zai":
		return ""
	case "giteeai":
		return ""
	case "docker-model-runner":
		return ""
	case "privatemode":
		return ""
	case "sambanova":
		return ""
	case "lemonade":
		return ""
	case "minimax":
		return ""
	case "qwen":
		return ""
	case "wenxin":
		return ""
	case "zhipu":
		return ""
	}
	return "gpt-4o-mini"
}

func parseAzureEndpoint(endpoint string) (resourceName, deployment string, err error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("parse azure endpoint: %w", err)
	}
	// Hostname: myresource.openai.azure.com
	host := u.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) < 4 || !strings.HasSuffix(host, ".openai.azure.com") {
		return "", "", fmt.Errorf("azure endpoint hostname must be {resourceName}.openai.azure.com")
	}
	resourceName = parts[0]

	// Path: /openai/deployments/{deployment}
	path := strings.Trim(u.Path, "/")
	pathParts := strings.Split(path, "/")
	if len(pathParts) >= 3 && pathParts[0] == "openai" && pathParts[1] == "deployments" {
		deployment = pathParts[2]
	}

	if deployment == "" {
		return "", "", fmt.Errorf("azure endpoint path must contain /openai/deployments/{deployment}")
	}
	return resourceName, deployment, nil
}
